# 模型重定向 Abilities 表自动填充功能

## 问题描述

之前的 new-api 实现中，模型重定向无法正常工作，因为：
1. 渠道选择在模型重定向之前执行
2. 渠道选择基于 `abilities` 表查询
3. `abilities` 表中只包含渠道模型列表中的模型，不包含重定向目标模型

## 解决方案

修改了 `model/ability.go` 和 `model/channel.go`，在生成 abilities 表记录时，**自动将模型重定向配置中的目标模型也添加到 abilities 表中**。

## 修改的文件

### 1. `model/channel.go`

新增 `getModelsWithMappingTargets()` 方法：
- 解析渠道的 `model_mapping` 配置
- 提取所有重定向的目标模型
- 返回原始模型 + 目标模型的完整列表

### 2. `model/ability.go`

修改了两个方法：
- `AddAbilities()` - 添加渠道时生成 abilities
- `UpdateAbilities()` - 更新渠道时重新生成 abilities

这两个方法现在都会调用 `getModelsWithMappingTargets()` 来获取完整的模型列表。

## 使用示例

### 配置

**渠道模型列表**：
```
gemini-3.1-pro
```

**模型重定向配置**：
```json
{
  "gemini-3.1-pro": "gemini-3-pro"
}
```

### 执行流程

1. **保存/更新渠道时**：
   - 系统自动解析 `model_mapping`
   - 发现重定向：`gemini-3.1-pro` → `gemini-3-pro`
   - 在 `abilities` 表中创建两条记录：
     - ✅ `gemini-3.1-pro` (原始模型)
     - ✅ `gemini-3-pro` (重定向目标)

2. **用户请求时**：
   ```
   POST /v1/chat/completions
   { "model": "gemini-3.1-pro" }
   ```

3. **渠道选择**：
   - 查询 `abilities` 表
   - ✅ 找到 `gemini-3.1-pro` 记录
   - ✅ 成功匹配到渠道

4. **模型重定向**：
   - 应用 `model_mapping`
   - 将模型名从 `gemini-3.1-pro` 改为 `gemini-3-pro`

5. **发送上游请求**：
   ```
   POST https://generativelanguage.googleapis.com/v1beta/chat/completions
   { "model": "gemini-3-pro" }
   ```

## 优势

✅ **干净的配置**
   - 渠道模型列表只包含对外提供的模型名
   - 不需要同时添加真实模型名和别名

✅ **自动同步**
   - 修改模型重定向配置后，abilities 表自动更新
   - 无需手动维护

✅ **向后兼容**
   - 不影响现有功能
   - 没有配置模型重定向的渠道行为不变

✅ **支持复杂场景**
   - 支持多对一映射：多个别名映射到同一个真实模型
   - 支持链式重定向（原有功能）

## 示例场景

### 场景 1：简单重定向

**配置**：
```
Models: gemini-3.1-pro
ModelMapping: {"gemini-3.1-pro": "gemini-3-pro"}
```

**abilities 表生成**：
- `gemini-3.1-pro` ✅
- `gemini-3-pro` ✅

### 场景 2：多别名

**配置**：
```
Models: gemini-latest,gemini-best
ModelMapping: {
  "gemini-latest": "gemini-2.0-flash",
  "gemini-best": "gemini-2.0-flash"
}
```

**abilities 表生成**：
- `gemini-latest` ✅
- `gemini-best` ✅
- `gemini-2.0-flash` ✅（去重，只生成一条）

### 场景 3：部分重定向

**配置**：
```
Models: gpt-4,gpt-4-turbo,my-custom-model
ModelMapping: {"my-custom-model": "gpt-4-turbo"}
```

**abilities 表生成**：
- `gpt-4` ✅
- `gpt-4-turbo` ✅（原本就在列表中，不重复）
- `my-custom-model` ✅

## 如何使用

### 新建渠道

1. 在渠道管理页面创建渠道
2. 在"模型"字段中只填写对外提供的模型名（如 `gemini-3.1-pro`）
3. 在"模型重定向"字段配置映射（如 `{"gemini-3.1-pro": "gemini-3-pro"}`）
4. 保存

### 更新现有渠道

1. 修改渠道的模型列表或模型重定向配置
2. 保存
3. abilities 表会自动重新生成

### 验证

查询 abilities 表：
```sql
SELECT group, model, channel_id 
FROM abilities 
WHERE channel_id = <your_channel_id>;
```

应该能看到原始模型和重定向目标模型都存在。

## 注意事项

⚠️ **重定向目标必须是上游 API 支持的真实模型**
   - 虽然系统会自动添加目标模型到 abilities 表
   - 但最终发送给上游 API 的模型名必须是有效的

⚠️ **循环重定向检测**
   - 原有的循环重定向检测仍然有效
   - 避免配置如 `{"A": "B", "B": "A"}` 的循环映射

⚠️ **模型重定向是渠道级别的**
   - 每个渠道可以有自己的模型重定向配置
   - 不同渠道的重定向互不影响

## 技术细节

### 实现原理

`getModelsWithMappingTargets()` 方法使用 map 来去重：
```go
modelSet := make(map[string]struct{})
// 1. 添加原始模型
// 2. 解析 model_mapping，添加目标模型
// 3. 返回去重后的列表
```

### 性能影响

- ✅ 只在保存/更新渠道时解析一次
- ✅ 不影响运行时性能
- ✅ abilities 表查询不受影响（索引仍然有效）

## 后续优化建议

1. **前端提示**：在配置模型重定向时，前端可以提示哪些目标模型会自动添加到 abilities 表

2. **日志记录**：在生成 abilities 时，记录哪些模型是从 model_mapping 中添加的

3. **验证目标模型**：可选地验证重定向目标模型是否在渠道类型的白名单中（虽然不强制）

## 总结

这个修改完美解决了模型重定向无法工作的问题，同时保持了配置的简洁性和系统的扩展性。

