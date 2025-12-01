package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var channelSyncLock sync.RWMutex

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	var channels []*Channel
	// 明确选择所有字段，确保 model_mapping 等字段被正确加载
	DB.Select("*").Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
	}
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			// 使用 getModelsWithMappingTargets 来包含重定向目标模型
			baseModels := strings.Split(channel.Models, ",")
			models := channel.getModelsWithMappingTargets(baseModels)
			
			// 强制日志：显示扩展后的模型列表
			if len(models) != len(baseModels) {
				common.SysLog(fmt.Sprintf("[Cache] Channel #%d: models expanded from %d to %d: %v -> %v, ModelMapping=%v",
					channel.Id, len(baseModels), len(models), baseModels, models, 
					func() string {
						if channel.ModelMapping == nil {
							return "nil"
						}
						return *channel.ModelMapping
					}()))
			}
			
			for _, model := range models {
				model = strings.TrimSpace(model)
				if model == "" {
					continue
				}
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	
	// 先处理多密钥轮询索引，使用旧的 channelsIDM
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	
	// 原子性地同时更新两个 map，确保一致性
	channelsIDM = newChannelId2channel
	group2model2channels = newGroup2model2channels
	
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, retry int) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannel(group, model, retry)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := group2model2channels[group][model]

	// 调试日志：显示查询参数和结果
	common.SysLog(fmt.Sprintf("[Query] GetRandomSatisfiedChannel: group=%s, model=%s, found=%d channels", group, model, len(channels)))
	
	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		if normalizedModel != model {
			channels = group2model2channels[group][normalizedModel]
			common.SysLog(fmt.Sprintf("[Query] Normalized model: %s -> %s, found=%d channels", model, normalizedModel, len(channels)))
		}
	}

	if len(channels) == 0 {
		// 调试：显示缓存中有哪些 group 和 model
		common.SysLog(fmt.Sprintf("[Query] Cache miss! Available groups: %v", func() []string {
			groups := make([]string, 0, len(group2model2channels))
			for g := range group2model2channels {
				groups = append(groups, g)
			}
			return groups
		}()))
		if models, ok := group2model2channels[group]; ok {
			common.SysLog(fmt.Sprintf("[Query] Available models in group '%s': %v", group, func() []string {
				modelList := make([]string, 0, len(models))
				for m := range models {
					modelList = append(modelList, m)
				}
				return modelList
			}()))
		}
		return nil, nil
	}

	if len(channels) == 1 {
		channelId := channels[0]
		if channel, ok := channelsIDM[channelId]; ok {
			common.SysLog(fmt.Sprintf("[Query] Found single channel: #%d, name=%s", channelId, channel.Name))
			return channel, nil
		}
		common.SysLog(fmt.Sprintf("[Query] ERROR: Channel #%d found in cache but not in channelsIDM! channelsIDM keys: %v", 
			channelId, func() []int {
				keys := make([]int, 0, len(channelsIDM))
				for k := range channelsIDM {
					keys = append(keys, k)
				}
				return keys
			}()))
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			common.SysLog(fmt.Sprintf("[Query] ERROR: Channel #%d not found in channelsIDM", channelId))
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	common.SysLog(fmt.Sprintf("[Query] Found %d unique priorities: %v, retry=%d", len(uniquePriorities), sortedUniquePriorities, retry))
	
	if retry >= len(uniquePriorities) {
		common.SysLog(fmt.Sprintf("[Query] Retry %d >= %d priorities, adjusting to %d", retry, len(uniquePriorities), len(uniquePriorities)-1))
		retry = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])
	common.SysLog(fmt.Sprintf("[Query] Target priority: %d", targetPriority))

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetPriority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	println("CacheUpdateChannel:", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)

	println("before:", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
	channelsIDM[channel.Id] = channel
	println("after :", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
}
