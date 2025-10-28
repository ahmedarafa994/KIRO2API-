package server

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"kiro2api/config"
	"kiro2api/converter"
	"kiro2api/logger"
	"kiro2api/parser"
	"kiro2api/types"
	"kiro2api/utils"

	"github.com/gin-gonic/gin"
)

// handleOpenAINonStreamRequest 处理OpenAI非流式请求
func handleOpenAINonStreamRequest(c *gin.Context, anthropicReq types.AnthropicRequest, token types.TokenInfo) {
	resp, err := executeCodeWhispererRequest(c, anthropicReq, token, false)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := utils.ReadHTTPResponse(resp.Body)
	if err != nil {
		handleResponseReadError(c, err)
		return
	}

	// 使用新的符合AWS规范的解析器
	compliantParser := parser.NewCompliantEventStreamParser()
	result, err := compliantParser.ParseResponse(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "响应解析失败"})
		return
	}

	// 转换为Anthropic格式
	contexts := []map[string]any{}
	allContent := result.GetCompletionText()
	sawToolUse := len(result.GetToolCalls()) > 0

	// 添加文本内容
	if allContent != "" {
		contexts = append(contexts, map[string]any{
			"type": "text",
			"text": allContent,
		})
	}

	// 添加工具调用
	for _, tool := range result.GetToolCalls() {
		contexts = append(contexts, map[string]any{
			"type":  "tool_use",
			"id":    tool.ID,
			"name":  tool.Name,
			"input": tool.Arguments,
		})
	}

	// 构建Anthropic响应
	inputContent, _ := utils.GetMessageContent(anthropicReq.Messages[0].Content)
	stopReason := func() string {
		if sawToolUse {
			return "tool_use"
		}
		return "end_turn"
	}()
	anthropicResp := map[string]any{
		"content":       contexts,
		"model":         anthropicReq.Model,
		"role":          "assistant",
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"type":          "message",
		"usage": map[string]any{
			"input_tokens":  len(inputContent),
			"output_tokens": len(allContent),
		},
	}

	// 转换为OpenAI格式
	openaiMessageId := fmt.Sprintf("chatcmpl-%s", time.Now().Format(config.MessageIDTimeFormat))
	openaiResp := converter.ConvertAnthropicToOpenAI(anthropicResp, anthropicReq.Model, openaiMessageId)

	// 下发OpenAI兼容非流式响应
	logger.Debug("下发OpenAI非流式响应",
		addReqFields(c,
			logger.String("direction", "downstream_send"),
			logger.Bool("saw_tool_use", sawToolUse),
		)...)
	c.JSON(http.StatusOK, openaiResp)
}

// handleOpenAIStreamRequest 处理OpenAI流式请求
func handleOpenAIStreamRequest(c *gin.Context, anthropicReq types.AnthropicRequest, token types.TokenInfo) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用nginx缓冲

	messageId := fmt.Sprintf("chatcmpl-%s", time.Now().Format(config.MessageIDTimeFormat))
	// 注入 message_id，便于统一日志会话标识
	c.Set("message_id", messageId)

	resp, err := executeCodeWhispererRequest(c, anthropicReq, token, true)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// 立即刷新响应头
	c.Writer.Flush()

	sender := &OpenAIStreamSender{}

	// 发送初始OpenAI事件
	initialEvent := map[string]any{
		"id":      messageId,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   anthropicReq.Model,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"role": "assistant",
				},
				"finish_reason": nil,
			},
		},
	}
	sender.SendEvent(c, initialEvent)

	// 创建符合AWS规范的流式解析器
	compliantParser := parser.NewCompliantEventStreamParser()

	// OpenAI 工具调用增量状态
	toolIndexByToolUseId := make(map[string]int)  // tool_use_id -> tool_calls 数组索引
	toolUseIdByBlockIndex := make(map[int]string) // 内容块 index -> tool_use_id
	nextToolIndex := 0
	sawToolUse := false
	sentFinal := false

	// 添加完整性跟踪
	totalBytesRead := 0
	messageCount := 0
	hasMoreData := true
	consecutiveErrors := 0
	const maxConsecutiveErrors = 3

	// 使用更大的缓冲区避免数据丢失
	buf := make([]byte, 8192) // 增加到8KB
	for hasMoreData {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			totalBytesRead += n
			consecutiveErrors = 0 // 重置错误计数

			events, parseErr := compliantParser.ParseStream(buf[:n])
			if parseErr != nil {
				// 在宽松模式下继续处理
				continue
			}
			messageCount += len(events)
			for _, event := range events {
				if event.Data != nil {
					if dataMap, ok := event.Data.(map[string]any); ok {
						switch dataMap["type"] {
						case "content_block_delta":
							if delta, ok := dataMap["delta"]; ok {
								if deltaMap, ok := delta.(map[string]any); ok {
									switch deltaMap["type"] {
									case "text_delta":
										if text, ok := deltaMap["text"]; ok {
											// 发送文本内容的增量
											contentEvent := map[string]any{
												"id":      messageId,
												"object":  "chat.completion.chunk",
												"created": time.Now().Unix(),
												"model":   anthropicReq.Model,
												"choices": []map[string]any{
													{
														"index": 0,
														"delta": map[string]any{
															"content": text.(string),
														},
														"finish_reason": nil,
													},
												},
											}
											sender.SendEvent(c, contentEvent)
										}
									case "input_json_delta":
										// 工具调用参数增量
										// 找到对应的tool_use和OpenAI tool_calls索引
										toolBlockIndex := 0
										if idxAny, ok := dataMap["index"]; ok {
											switch v := idxAny.(type) {
											case int:
												toolBlockIndex = v
											case int32:
												toolBlockIndex = int(v)
											case int64:
												toolBlockIndex = int(v)
											case float64:
												toolBlockIndex = int(v)
											}
										}
										if toolUseId, ok := toolUseIdByBlockIndex[toolBlockIndex]; ok {
											if toolIdx, ok := toolIndexByToolUseId[toolUseId]; ok {
												var partial string
												if pj, ok := deltaMap["partial_json"]; ok {
													switch s := pj.(type) {
													case string:
														partial = s
													case *string:
														if s != nil {
															partial = *s
														}
													}
												}
												if partial != "" {
													toolDelta := map[string]any{
														"id":      messageId,
														"object":  "chat.completion.chunk",
														"created": time.Now().Unix(),
														"model":   anthropicReq.Model,
														"choices": []map[string]any{
															{
																"index": 0,
																"delta": map[string]any{
																	"tool_calls": []map[string]any{
																		{
																			"index": toolIdx,
																			"type":  "function",
																			"function": map[string]any{
																				"arguments": partial,
																			},
																		},
																	},
																},
																"finish_reason": nil,
															},
														},
													}
													sender.SendEvent(c, toolDelta)
												}
											}
										}
									}
								}
							}
						case "content_block_start":
							if contentBlock, ok := dataMap["content_block"]; ok {
								if blockMap, ok := contentBlock.(map[string]any); ok {
									if blockType, _ := blockMap["type"].(string); blockType == "tool_use" {
										toolUseId, _ := blockMap["id"].(string)
										toolName, _ := blockMap["name"].(string)
										// 获取内容块索引
										toolBlockIndex := 0
										if idxAny, ok := dataMap["index"]; ok {
											switch v := idxAny.(type) {
											case int:
												toolBlockIndex = v
											case int32:
												toolBlockIndex = int(v)
											case int64:
												toolBlockIndex = int(v)
											case float64:
												toolBlockIndex = int(v)
											}
										}
										if toolUseId != "" {
											if _, exists := toolIndexByToolUseId[toolUseId]; !exists {
												toolIndexByToolUseId[toolUseId] = nextToolIndex
												nextToolIndex++
											}
											toolUseIdByBlockIndex[toolBlockIndex] = toolUseId
											sawToolUse = true
											toolIdx := toolIndexByToolUseId[toolUseId]
											// 发送OpenAI工具调用开始增量
											toolStart := map[string]any{
												"id":      messageId,
												"object":  "chat.completion.chunk",
												"created": time.Now().Unix(),
												"model":   anthropicReq.Model,
												"choices": []map[string]any{
													{
														"index": 0,
														"delta": map[string]any{
															"tool_calls": []map[string]any{
																{
																	"index": toolIdx,
																	"id":    toolUseId,
																	"type":  "function",
																	"function": map[string]any{
																		"name":      toolName,
																		"arguments": "",
																	},
																},
															},
														},
														"finish_reason": nil,
													},
												},
											}
											sender.SendEvent(c, toolStart)
										}
									}
								}
							}
						case "message_delta":
							// 将Claude的tool_use结束映射为OpenAI的finish_reason=tool_calls
							if sawToolUse && !sentFinal {
								if delta, ok := dataMap["delta"].(map[string]any); ok {
									if sr, ok := delta["stop_reason"].(string); ok && sr == "tool_use" {
										endEvent := map[string]any{
											"id":      messageId,
											"object":  "chat.completion.chunk",
											"created": time.Now().Unix(),
											"model":   anthropicReq.Model,
											"choices": []map[string]any{
												{
													"index":         0,
													"delta":         map[string]any{},
													"finish_reason": "tool_calls",
												},
											},
										}
										sender.SendEvent(c, endEvent)
										sentFinal = true
									}
								}
							}
						case "content_block_stop":
							// 忽略，最终结束由message_delta驱动
						}
					}
				}
				c.Writer.Flush()
			}
		}

		// 错误处理
		if err != nil {
			if err == io.EOF {
				// 正常结束
				hasMoreData = false
			} else if err == io.ErrUnexpectedEOF {
				// 意外结束，尝试恢复
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					// 连续错误过多，停止
					hasMoreData = false
				} else {
					// 使用select支持context取消
					select {
					case <-time.After(config.RetryDelay):
						continue
					case <-c.Request.Context().Done():
						hasMoreData = false
					}
				}
			} else {
				// 其他错误
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					hasMoreData = false
				} else {
					// 尝试继续读取
					continue
				}
			}
		}
	}

	// 确保发送了结束原因（如果还没有发送）
	if !sentFinal && messageCount > 0 {
		finishReason := "stop"
		if sawToolUse {
			finishReason = "tool_calls"
		}

		finalEvent := map[string]any{
			"id":      messageId,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   anthropicReq.Model,
			"choices": []map[string]any{
				{
					"index":         0,
					"delta":         map[string]any{},
					"finish_reason": finishReason,
				},
			},
		}
		sender.SendEvent(c, finalEvent)
		c.Writer.Flush()
	}

	// 发送结束标记
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}
