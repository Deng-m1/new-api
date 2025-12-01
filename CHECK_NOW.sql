-- 立即执行这个查询来诊断问题
-- ==========================================

-- 1. 检查渠道 #1 的配置
SELECT 
    id,
    name,
    type,
    status,
    "group",
    models,
    model_mapping
FROM channels 
WHERE id = 1;

-- 2. 检查 abilities 表中渠道 #1 的所有记录
SELECT 
    channel_id,
    "group",
    model,
    enabled,
    priority
FROM abilities 
WHERE channel_id = 1
ORDER BY model;

-- 3. 查询能匹配 gpt-5 的所有渠道
SELECT 
    a.channel_id,
    c.name,
    a."group",
    a.model,
    a.enabled,
    c.status as channel_status
FROM abilities a
JOIN channels c ON c.id = a.channel_id
WHERE a.model = 'gpt-5'
ORDER BY a.channel_id;

-- 4. 查询能匹配 openai/gpt-5 的所有渠道
SELECT 
    a.channel_id,
    c.name,
    a."group",
    a.model,
    a.enabled,
    c.status as channel_status
FROM abilities a
JOIN channels c ON c.id = a.channel_id
WHERE a.model = 'openai/gpt-5'
ORDER BY a.channel_id;

-- 5. 比较 gemini 和 gpt 渠道的配置
SELECT 
    c.id,
    c.name,
    c.status,
    c."group",
    c.models,
    COUNT(a.model) as ability_count
FROM channels c
LEFT JOIN abilities a ON a.channel_id = c.id AND a.enabled = true
WHERE c.id IN (1, 11)
GROUP BY c.id, c.name, c.status, c."group", c.models
ORDER BY c.id;


