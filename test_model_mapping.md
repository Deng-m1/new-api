# 模型重定向功能测试指南

## 修改内容总结

已修改的文件：
1. ✅ `model/channel.go` - 添加 `getModelsWithMappingTargets()` 方法（含调试日志）
2. ✅ `model/ability.go` - 修改 `AddAbilities()` 和 `UpdateAbilities()`
3. ✅ `model/channel_cache.go` - 修改 `InitChannelCache()` 使用 `Select("*")` 并调用 `getModelsWithMappingTargets()`

## 测试步骤

### 1. 重新编译

```bash
# 在项目根目录
go build
```

### 2. 启用调试模式（可选但推荐）

在启动前设置环境变量：

**Windows PowerShell**:
```powershell
$env:DEBUG="true"
./new-api.exe
```

**Linux/Mac**:
```bash
DEBUG=true ./new-api
```

### 3. 配置测试渠道

#### 渠道配置示例

**基本信息**：
- 渠道类型: OpenAI
- 渠道名称: 测试渠道
- 分组: default
- 模型列表: `gpt-5`（只填这一个！）

**模型重定向**:
```json
{
  "gpt-5": "openai/gpt-5"
}
```

**或者**:
```json
{
  "gpt-5": "gpt-4"
}
```

### 4. 保存后检查日志

保存渠道后，应该能在日志中看到（如果启用了 DEBUG）：

```
[SYS] ... | [ModelMapping] Channel #1: gpt-5 -> openai/gpt-5 (added both to abilities)
[SYS] ... | [ModelMapping] Channel #1: Expanded models from [gpt-5] to [gpt-5 openai/gpt-5]
[SYS] ... | channels synced from database
```

### 5. 验证 Abilities 表

**PostgreSQL**:
```sql
SELECT channel_id, "group", model, enabled 
FROM abilities 
WHERE channel_id = 1
ORDER BY model;
```

**预期结果**:
```
channel_id | group   | model         | enabled
-----------+---------+---------------+---------
1          | default | gpt-5         | true
1          | default | openai/gpt-5  | true
```

### 6. 测试请求

```bash
# 测试原始模型名
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5",
    "messages": [{"role": "user", "content": "hi"}]
  }'
```

### 7. 检查日志输出

**成功的情况**：
```
[INFO] ... | 用户 1 额度充足且为无限额度令牌, 信任且不需要预扣费
[INFO] ... | 成功匹配到渠道 #1
(没有 "no available channels" 错误)
```

**失败的情况（需要修复）**：
```
[ERR] ... | no available channels for model gpt-5
```

## 常见问题排查

### 问题 1: 保存后仍然报 "no available channels"

**原因**: 缓存可能没有正确刷新

**解决方案**:
1. 检查是否启用了内存缓存（查看日志中是否有 "syncing channels from database"）
2. 重启服务
3. 或调用修复 API:
   ```bash
   curl -X POST http://localhost:3000/api/channel/fix \
     -H "Authorization: Bearer your-admin-token"
   ```

### 问题 2: 日志中没有 [ModelMapping] 信息

**原因**: DEBUG 模式未启用

**解决方案**: 
```bash
# Windows
$env:DEBUG="true"
./new-api.exe

# Linux/Mac
DEBUG=true ./new-api
```

### 问题 3: Abilities 表中只有原始模型

**原因**: `ModelMapping` 字段没有被正确读取

**检查**:
```sql
SELECT id, name, models, model_mapping 
FROM channels 
WHERE id = 1;
```

确认 `model_mapping` 字段不是 NULL 或空字符串。

### 问题 4: Authorization header 错误

这是**另一个问题**，与模型映射无关：

```
[ERR] ... | do request failed: ... invalid header field value for "Authorization"
```

**检查**:
1. 渠道的 Key 是否包含换行符或特殊字符
2. 渠道的 Key 格式是否正确

## 验证脚本

### 检查配置脚本（PostgreSQL）

```sql
-- 1. 检查渠道配置
SELECT 
    id,
    name,
    type,
    status,
    models,
    model_mapping,
    "group"
FROM channels 
WHERE id = 1;

-- 2. 检查 abilities 表
SELECT 
    channel_id,
    "group",
    model,
    enabled,
    priority
FROM abilities 
WHERE channel_id = 1
ORDER BY model;

-- 3. 统计每个渠道的模型数量
SELECT 
    c.id,
    c.name,
    c.models as channel_models,
    COUNT(a.model) as ability_count,
    STRING_AGG(a.model, ', ' ORDER BY a.model) as ability_models
FROM channels c
LEFT JOIN abilities a ON a.channel_id = c.id
WHERE c.id = 1
GROUP BY c.id, c.name, c.models;
```

### 测试所有功能

```bash
#!/bin/bash

API_URL="http://localhost:3000"
TOKEN="sk-your-token"

echo "测试 1: 请求原始模型名 (gpt-5)"
curl -s -X POST $API_URL/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5",
    "messages": [{"role": "user", "content": "test"}],
    "max_tokens": 10
  }' | jq -r '.error.message // "成功"'

echo ""
echo "测试 2: 请求重定向目标模型 (openai/gpt-5)"
curl -s -X POST $API_URL/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-5",
    "messages": [{"role": "user", "content": "test"}],
    "max_tokens": 10
  }' | jq -r '.error.message // "成功"'
```

## 成功标志

✅ **配置阶段**:
- Abilities 表中同时存在原始模型和目标模型
- 日志显示模型扩展信息（DEBUG 模式）
- 缓存成功同步

✅ **运行阶段**:
- 请求原始模型名能匹配到渠道
- 请求目标模型名也能匹配到渠道
- 上游 API 收到的是重定向后的模型名

## 预期行为

### 配置
```
渠道模型: gpt-5
重定向: {"gpt-5": "gpt-4"}
```

### 结果
```
用户请求 "gpt-5"
  ↓
查询 abilities 表: ✅ 找到 gpt-5 (channel_id=1)
  ↓
选择渠道 #1 ✅
  ↓
应用模型重定向: gpt-5 -> gpt-4
  ↓
发送给上游: model="gpt-4" ✅
```

### 同时支持
```
用户请求 "gpt-4" (目标模型)
  ↓
查询 abilities 表: ✅ 找到 gpt-4 (channel_id=1，自动添加)
  ↓
选择渠道 #1 ✅
  ↓
无需重定向
  ↓
发送给上游: model="gpt-4" ✅
```

## 回滚方案

如果出现问题，可以快速回滚：

```bash
git checkout HEAD -- model/channel.go model/ability.go model/channel_cache.go
go build
```

然后重启服务。


