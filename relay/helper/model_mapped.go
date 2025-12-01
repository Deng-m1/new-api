package helper

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request) error {
	// map model name
	modelMapping := c.GetString("model_mapping")
	common.SysLog(fmt.Sprintf("[ModelMapping] Start: OriginModel=%s, modelMapping=%s", info.OriginModelName, modelMapping))
	
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := json.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			common.SysLog(fmt.Sprintf("[ModelMapping] Failed to unmarshal: %v", err))
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := info.OriginModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			if mappedModel, exists := modelMap[currentModel]; exists && mappedModel != "" {
				common.SysLog(fmt.Sprintf("[ModelMapping] Found mapping: %s -> %s", currentModel, mappedModel))
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							info.IsModelMapped = false
							common.SysLog(fmt.Sprintf("[ModelMapping] Self-reference, no mapping applied"))
							return nil
						} else {
							info.IsModelMapped = true
							break
						}
					}
					common.SysLog(fmt.Sprintf("[ModelMapping] Cycle detected"))
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
			common.SysLog(fmt.Sprintf("[ModelMapping] Final result: %s -> %s", info.OriginModelName, info.UpstreamModelName))
		} else {
			common.SysLog(fmt.Sprintf("[ModelMapping] No mapping found for: %s", info.OriginModelName))
		}
	} else {
		common.SysLog(fmt.Sprintf("[ModelMapping] No model_mapping configured"))
	}
	if request != nil {
		request.SetModelName(info.UpstreamModelName)
		common.SysLog(fmt.Sprintf("[ModelMapping] Set request model to: %s", info.UpstreamModelName))
	}
	return nil
}
