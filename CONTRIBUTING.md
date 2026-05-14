# 贡献指南

感谢你对 RuleFlow 的关注！以下是参与贡献的基本流程。

## 提交 Issue

- 报告 Bug 前请先搜索现有 Issue，避免重复
- 使用 Issue 模板，尽量提供完整的复现步骤、环境信息和错误日志
- 功能建议请说明使用场景和预期效果

## 提交 Pull Request

1. Fork 仓库，基于 `main` 分支新建功能分支
2. 保持单个 PR 只做一件事，方便 Review
3. Commit Message 遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范，例如：
   - `feat: 支持 TUIC v5 协议解析`
   - `fix: 修复 Surge dialer-proxy 生成错误`
   - `docs: 补充 API 参考示例`
4. 确保 `make test` 通过后再提交
5. PR 描述中说明改动目的和测试方式

## 本地开发

```bash
cp .env.example .env
# 编辑 .env 填写数据库等信息

make migrate   # 初始化数据库
make run       # 启动服务
make test      # 运行测试
```

## 代码风格

- 遵循标准 Go 风格，提交前运行 `gofmt`
- 新增功能建议配套测试用例

## 许可证

提交代码即表示你同意将代码以 [MIT License](LICENSE) 授权给本项目。
