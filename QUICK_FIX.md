# 快速修复指南

## 当前问题

你遇到的问题是**缓存不稳定**，交替出现：
- ✅ 有时能匹配到渠道（但 Authorization 失败）
- ❌ 有时找不到渠道 "no available channels for model gpt-5"

## 🚀 快速修复步骤

### 1. 确认代码已更新

```bash
# 查看最后修改的文件
git status

# 应该看到：
#   modified:   model/ability.go
#   modified:   model/channel.go
#   modified:   model/channel_cache.go
```

### 2. 完全重新编译

```bash
# 清理旧的编译文件
go clean

# 重新编译（会显示编译进度）
go build -v

# 检查编译时间（应该是刚才的时间）
# Windows:
dir new-api.exe

# Linux/Mac:
ls -lh new-api
```

### 3. 完全停止并重启

```bash
# 方式 1: 如果在前台运行
Ctrl+C (停止)
等待 5 秒
./new-api.exe  # 或 ./new-api

# 方式 2: 如果在后台运行
# Windows:
taskkill /F /IM new-api.exe
./new-api.exe

# Linux/Mac:
pkill new-api
./new-api
```

### 4. 检查启动日志

重启后，应该看到类似的日志：

```
[SYS] ... | channels synced from database
[SYS] ... | [Cache] Channel #1: models expanded from 1 to 2: [gpt-5] -> [gpt-5 openai/gpt-5], ModelMapping={"gpt-5":"openai/gpt-5"}
```

**如果没有看到 `[Cache]` 日志**：说明 ModelMapping 字段为空或 NULL！

### 5. 验证数据库

```sql
-- 检查 model_mapping 字段
SELECT 
    id,
    name,
    models,
    model_mapping,
    CASE 
        WHEN model_mapping IS NULL THEN '❌ NULL'
        WHEN model_mapping = '' THEN '❌ EMPTY'
        WHEN model_mapping = '{}' THEN '❌ EMPTY_JSON'
        ELSE '✅ HAS_VALUE'
    END as status
FROM channels 
WHERE id = 1;
```

**如果 status 不是 `✅ HAS_VALUE`**：
1. 在管理界面重新配置模型重定向
2. 确保 JSON 格式正确：`{"gpt-5":"openai/gpt-5"}`
3. 保存

### 6. 检查 Abilities 表

```sql
-- 应该看到两条记录
SELECT model, enabled 
FROM abilities 
WHERE channel_id = 1
ORDER BY model;

-- 预期输出：
--   model         | enabled
-- ----------------+---------
--   gpt-5         | true
--   openai/gpt-5  | true
```

**如果只有一条记录**：说明代码未生效，需要重新编译。

### 7. 测试请求

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-your-token" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5","messages":[{"role":"user","content":"hi"}],"max_tokens":5}'
```

---

## 🔍 如果仍然失败

### 检查点 A: 代码是否真的更新了

在 `model/channel_cache.go` 第 47 行附近，应该能看到：

```go
// 使用 getModelsWithMappingTargets 来包含重定向目标模型
baseModels := strings.Split(channel.Models, ",")
models := channel.getModelsWithMappingTargets(baseModels)
```

如果没有这段代码，说明文件没有更新。

### 检查点 B: 是否使用了正确的可执行文件

```bash
# 检查当前目录
pwd

# 检查可执行文件位置
which new-api  # Linux/Mac
where new-api  # Windows

# 确保运行的是当前目录下的文件
./new-api.exe  # Windows (注意 ./ 前缀)
./new-api      # Linux/Mac (注意 ./ 前缀)
```

### 检查点 C: 环境变量检查

```bash
# 检查是否启用了内存缓存（默认启用）
# 如果日志中没有 "syncing channels from database"
# 说明内存缓存被禁用了

# 确保没有设置这个环境变量
echo $MEMORY_CACHE_ENABLED  # 应该为空或 true
```

---

## 🎯 最简单的验证方法

1. **停止服务**
2. **运行这个 SQL**:
   ```sql
   SELECT model FROM abilities WHERE channel_id = 1 ORDER BY model;
   ```
3. **如果只看到 `gpt-5`**：代码未生效
4. **如果看到 `gpt-5` 和 `openai/gpt-5`**：代码已生效
5. **重启服务**
6. **查看日志中是否有 `[Cache]` 开头的日志**
7. **测试请求**

---

## ⚡ 紧急回滚

如果需要回滚到修改前：

```bash
git checkout HEAD -- model/ability.go model/channel.go model/channel_cache.go
go build
# 重启服务
```

---

## 📞 需要提供的信息

如果仍然不工作，请提供：

1. **启动日志中的这些行**：
   ```
   [SYS] ... | channels synced from database
   [SYS] ... | [Cache] Channel #1: ...
   ```

2. **SQL 查询结果**：
   ```sql
   SELECT id, models, model_mapping FROM channels WHERE id = 1;
   SELECT model FROM abilities WHERE channel_id = 1 ORDER BY model;
   ```

3. **编译信息**：
   ```bash
   go version
   go build -v 2>&1 | grep model
   ```

4. **测试请求的完整错误信息**


