-- ============================================
-- 模型重定向功能诊断脚本 (PostgreSQL)
-- ============================================

-- 1. 检查渠道配置
\echo '========== 1. 渠道配置 =========='
SELECT 
    id,
    name,
    type,
    status,
    "group",
    models,
    model_mapping,
    CASE 
        WHEN model_mapping IS NULL THEN 'NULL'
        WHEN model_mapping = '' THEN 'EMPTY'
        WHEN model_mapping = '{}' THEN 'EMPTY_JSON'
        ELSE 'HAS_VALUE'
    END as mapping_status,
    LENGTH(model_mapping) as mapping_length
FROM channels 
WHERE id = 1;

-- 2. 检查 abilities 表中的记录
\echo ''
\echo '========== 2. Abilities 表记录 =========='
SELECT 
    channel_id,
    "group",
    model,
    enabled,
    priority,
    weight
FROM abilities 
WHERE channel_id = 1
ORDER BY model;

-- 3. 检查每个模型的渠道数量
\echo ''
\echo '========== 3. 模型渠道统计 =========='
SELECT 
    model,
    COUNT(*) as channel_count,
    STRING_AGG(channel_id::text, ', ' ORDER BY channel_id) as channel_ids
FROM abilities 
WHERE model IN ('gpt-5', 'openai/gpt-5', 'gpt-4')
  AND enabled = true
GROUP BY model;

-- 4. 对比渠道配置和 abilities
\echo ''
\echo '========== 4. 配置 vs Abilities 对比 =========='
SELECT 
    c.id,
    c.name,
    c.models as channel_models,
    c.model_mapping,
    COUNT(DISTINCT a.model) as ability_model_count,
    STRING_AGG(DISTINCT a.model, ', ' ORDER BY a.model) as ability_models
FROM channels c
LEFT JOIN abilities a ON a.channel_id = c.id AND a.enabled = true
WHERE c.id = 1
GROUP BY c.id, c.name, c.models, c.model_mapping;

-- 5. 查找所有启用的渠道和模型
\echo ''
\echo '========== 5. 所有启用的渠道和模型 =========='
SELECT 
    c.id as channel_id,
    c.name as channel_name,
    c.status,
    a."group",
    a.model,
    a.enabled
FROM channels c
LEFT JOIN abilities a ON a.channel_id = c.id
WHERE c.status = 1
  AND (a.model LIKE '%gpt-5%' OR a.model LIKE '%gpt-4%')
ORDER BY c.id, a.model;

-- 6. 检查是否有重复或冲突的记录
\echo ''
\echo '========== 6. 重复检查 =========='
SELECT 
    "group",
    model,
    channel_id,
    COUNT(*) as count
FROM abilities
WHERE channel_id = 1
GROUP BY "group", model, channel_id
HAVING COUNT(*) > 1;

-- 7. 原始数据导出（用于调试）
\echo ''
\echo '========== 7. 原始数据 =========='
\echo 'Channel Raw Data:'
SELECT 
    id,
    name,
    models,
    model_mapping::text
FROM channels 
WHERE id = 1;

\echo ''
\echo 'Abilities Raw Data:'
SELECT * FROM abilities WHERE channel_id = 1;


