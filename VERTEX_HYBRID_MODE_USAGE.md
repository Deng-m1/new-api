# Vertex AI 混合模式使用指南

## 三种认证模式说明

### 1. JSON 模式（Service Account）
- **密钥类型**：选择 "JSON"
- **密钥输入方式**：上传 JSON 文件或手动输入
- **鉴权方式**：使用 Bearer Token
- **URL 格式**：包含 projects/locations 路径
- **支持批量创建**：✅ 是

### 2. API Key 模式（快速模式）
- **密钥类型**：选择 "API Key"  
- **密钥输入方式**：直接输入 API Key
- **鉴权方式**：使用 ?key= 查询参数
- **URL 格式**：不包含 projects/locations 路径
- **支持批量创建**：❌ 否

### 3. API Key (with Project) 模式（混合模式）⭐ 新增
- **密钥类型**：选择 "API Key (with Project)"
- **密钥输入方式**：直接输入 API Key
- **额外字段**：需要填写 Project ID
- **鉴权方式**：使用 ?key= 查询参数
- **URL 格式**：包含 projects/locations 路径
- **支持批量创建**：❌ 否

## 使用步骤（API Key with Project 模式）

1. **点击"新增渠道"**
2. **选择渠道类型**：Vertex AI
3. **填写渠道名称**：给渠道起个名字
4. **选择密钥格式**：选择 "API Key (with Project)"
5. **填写密钥**：在"密钥"输入框直接输入您的 API Key
   - 示例：`AIzaSyDxxxxxxxxxxxxxxxxxxxxxxx`
6. **填写 Project ID**：在新出现的"Project ID"输入框输入
   - 示例：`my-project-123456`
7. **填写 API 地区**：在"API 地区"输入框填写 location
   - 示例：`{"default": "global"}` 或 `{"default": "us-central1"}`
8. **选择模型**：选择要使用的模型
9. **保存**：点击提交

## 常见问题

### Q1: 为什么还是显示"请上传密钥文件"？
**A:** 请确保：
1. 已经选择了 "API Key (with Project)" 选项（不是 "JSON"）
2. 页面已经刷新，密钥格式切换已生效
3. 没有勾选"批量创建"复选框（该模式不支持批量创建）

### Q2: 为什么看不到聚合模式、轮询等选项？
**A:** 这些选项在密钥输入框下方的额外选项区域。确保：
1. 密钥格式已正确切换到 "API Key (with Project)"
2. 页面已完全加载

### Q3: Project ID 格式是什么？
**A:** Project ID 是您的 GCP 项目 ID，通常格式如：
- `my-project-123456`
- `production-app-001`
- 只包含小写字母、数字和连字符

## 故障排查

如果遇到问题，请按以下步骤操作：

1. **刷新页面**，重新打开创建渠道对话框
2. **清除浏览器缓存**
3. **查看浏览器控制台**，看是否有 JavaScript 错误
4. **确认选择了正确的密钥格式**：应该是 "API Key (with Project)"，不是 "API Key" 或 "JSON"
5. **检查是否有批量创建复选框被选中**，如果有，取消选中

## 技术细节

### URL 生成逻辑

根据选择的模式，系统会生成不同的 API URL：

**JSON 模式：**
```
https://aiplatform.googleapis.com/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:streamGenerateContent
Authorization: Bearer {token}
```

**API Key 模式：**
```
https://aiplatform.googleapis.com/v1/publishers/google/models/{model}:streamGenerateContent?key={API_KEY}
```

**API Key (with Project) 模式：**
```
https://aiplatform.googleapis.com/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:streamGenerateContent?key={API_KEY}
```

### 前端状态管理

- `batch`：批量创建模式标志（API Key 模式下强制为 false）
- `vertex_key_type`：密钥类型（'json' | 'api_key' | 'api_key_with_project'）
- `vertex_project_id`：Project ID（仅 api_key_with_project 模式需要）
- `useManualInput`：手动输入模式（仅 JSON 模式有效）

## 调试建议

如果前端显示不正确，可以打开浏览器开发者工具，在 Console 中运行：

```javascript
// 查看当前表单状态
console.log('Form inputs:', formApiRef.current?.getValues());

// 查看 batch 状态
console.log('Batch mode:', batch);

// 查看密钥类型
console.log('Vertex key type:', inputs.vertex_key_type);
```


