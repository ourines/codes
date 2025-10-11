# EnvSwitch - macOS 环境变量管理工具 PRD

## 1. 产品概述

### 1.1 产品定位

EnvSwitch 是一款轻量级的 macOS 状态栏应用，专为开发者设计，用于可视化管理和快速切换不同的环境变量配置。特别适合使用 Claude Code、Cursor 等 AI 编程工具的开发者。

### 1.2 核心价值

- **效率提升**：一键切换环境配置，无需手动编辑配置文件
- **降低错误**：可视化管理，避免手动修改引入的错误
- **便捷管理**：集中管理多个项目的环境变量配置

### 1.3 目标用户

- 频繁在多个项目间切换的开发者
- 需要维护开发/测试/生产多套环境的开发者
- 使用 AI 编程工具需要不同 API 配置的开发者

---

## 2. 功能需求

### 2.1 核心功能

#### 2.1.1 配置管理

- **创建配置**：添加新的环境变量配置集
- **编辑配置**：修改现有配置的名称、描述和环境变量
- **删除配置**：移除不需要的配置
- **复制配置**：基于现有配置创建副本

#### 2.1.2 环境变量操作

- **激活配置**：将选中的配置应用到系统环境
- **快速切换**：通过状态栏菜单快速切换配置
- **查看当前**：显示当前激活的配置
- **环境变量预览**：查看配置中的所有环境变量

#### 2.1.3 配置同步

- **导出配置**：将配置导出为 JSON 文件
- **导入配置**：从文件导入配置
- **配置模板**：提供常用框架的配置模板

### 2.2 辅助功能

#### 2.2.1 应用设置

- **Shell 类型选择**：支持 zsh、bash
- **自动应用**：切换配置时自动应用到 shell
- **启动选项**：开机自启动
- **通知设置**：配置切换通知

#### 2.2.2 安全与验证

- **配置验证**：检查环境变量格式
- **冲突检测**：提示重复的变量名
- **备份恢复**：自动备份配置

---

## 3. 技术架构

### 3.1 技术栈

- **开发语言**：Swift
- **UI 框架**：SwiftUI + AppKit (状态栏)
- **存储方案**：JSON 文件 + UserDefaults
- **部署目标**：macOS 13.0+

### 3.2 架构设计

```
┌─────────────────────────────────────────┐
│           Presentation Layer            │
│  ┌─────────────┐      ┌──────────────┐  │
│  │  Menu Bar   │      │  Main Window │  │
│  │   (AppKit)  │      │   (SwiftUI)  │  │
│  └─────────────┘      └──────────────┘  │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│           Business Logic Layer          │
│  ┌────────────┐  ┌──────────────────┐   │
│  │  Profile   │  │  Environment     │   │
│  │  Manager   │  │  Applier         │   │
│  └────────────┘  └──────────────────┘   │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│              Data Layer                 │
│  ┌────────────┐  ┌──────────────────┐   │
│  │   Local    │  │    File System   │   │
│  │  Storage   │  │    Operations    │   │
│  └────────────┘  └──────────────────┘   │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│           System Integration            │
│         Shell / Process / FS            │
└─────────────────────────────────────────┘
```

### 3.3 数据模型

```swift
// 环境变量配置
struct EnvProfile: Codable, Identifiable {
    let id: UUID
    var name: String
    var description: String?
    var variables: [EnvVariable]
    var isActive: Bool
    var createdAt: Date
    var updatedAt: Date
}

// 环境变量键值对
struct EnvVariable: Codable, Identifiable {
    let id: UUID
    var key: String
    var value: String
    var description: String?
}

// 应用设置
struct AppSettings: Codable {
    var shellType: ShellType  // zsh, bash
    var autoApply: Bool
    var showNotifications: Bool
    var launchAtLogin: Bool
}

enum ShellType: String, Codable {
    case zsh
    case bash
}
```

### 3.4 环境变量应用策略

**推荐方案：动态脚本注入**

1. 在 `~/.envswitch/` 目录创建激活脚本 `active.sh`
2. 用户在 `.zshrc` 或 `.bashrc` 中添加：`source ~/.envswitch/active.sh`
3. 应用切换配置时，重新生成 `active.sh` 内容
4. 用户在新终端或执行 `source ~/.zshrc` 后生效

**优点**：

- 不污染用户的 shell 配置文件
- 切换快速，只需修改一个文件
- 支持立即生效（通过执行 shell 脚本）

---

## 4. UI 设计原型 (ASCII)

### 4.1 状态栏菜单

```
┌─────────────────────────────────────────┐
│  ◉ EnvSwitch                            │
├─────────────────────────────────────────┤
│  Current: Development ✓                 │
├─────────────────────────────────────────┤
│  ○ Development              ⌘ 1         │
│  ◉ Production                ⌘ 2         │
│  ○ Staging                   ⌘ 3         │
│  ○ Claude Code - OpenAI      ⌘ 4         │
├─────────────────────────────────────────┤
│  + New Profile...                       │
│  ⚙  Manage Profiles...                  │
├─────────────────────────────────────────┤
│  Preferences...              ⌘ ,        │
│  Quit EnvSwitch              ⌘ Q        │
└─────────────────────────────────────────┘
```

### 4.2 配置列表窗口

```
┌─────────────────────────────────────────────────────────┐
│  EnvSwitch                                    ☐  ⊡  ✕   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │  🔍 Search profiles...                            │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │  ◉  Development                        [Activate] │ │
│  │      API_KEY=dev-123, DB_HOST=localhost           │ │
│  │      5 variables                                  │ │
│  │                                     [Edit] [Copy]  │ │
│  ├───────────────────────────────────────────────────┤ │
│  │  ○  Production                         [Activate] │ │
│  │      API_KEY=prod-*****, DB_HOST=prod.db          │ │
│  │      8 variables                                  │ │
│  │                                     [Edit] [Copy]  │ │
│  ├───────────────────────────────────────────────────┤ │
│  │  ○  Staging                            [Activate] │ │
│  │      API_KEY=staging-*****, DB_HOST=staging.db    │ │
│  │      6 variables                                  │ │
│  │                                     [Edit] [Copy]  │ │
│  ├───────────────────────────────────────────────────┤ │
│  │  ○  Claude Code - OpenAI               [Activate] │ │
│  │      ANTHROPIC_API_KEY=sk-ant-****                │ │
│  │      3 variables                                  │ │
│  │                                     [Edit] [Copy]  │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  [+ New Profile]  [Import]  [Export All]               │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 4.3 配置编辑窗口

```
┌─────────────────────────────────────────────────────────┐
│  Edit Profile: Development                    ☐  ⊡  ✕   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Profile Name *                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │  Development                                      │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  Description                                            │
│  ┌───────────────────────────────────────────────────┐ │
│  │  Development environment configuration            │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  Environment Variables                                  │
│  ┌───────────────────────────────────────────────────┐ │
│  │  KEY              VALUE              DESCRIPTION   │ │
│  ├───────────────────────────────────────────────────┤ │
│  │  API_KEY          dev-123-abc        API Key      │ │
│  │  DB_HOST          localhost          Database     │ │
│  │  DB_PORT          5432               DB Port      │ │
│  │  NODE_ENV         development        Environment  │ │
│  │  DEBUG            true                Debug Mode   │ │
│  │                                                    │ │
│  │  [+ Add Variable]                                 │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │  ⚠  Validation:                                 │   │
│  │  • Variable names must be uppercase             │   │
│  │  • No duplicate keys                            │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│                          [Cancel]  [Save Changes]       │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 4.4 添加/编辑环境变量行

```
┌─────────────────────────────────────────────────────────┐
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │ API_KEY     │  │ dev-123-abc  │  │ API Key      │ [×]│
│  └─────────────┘  └──────────────┘  └──────────────┘   │
└─────────────────────────────────────────────────────────┘
    ^KEY            ^VALUE            ^DESCRIPTION      ^Delete
```

### 4.5 偏好设置窗口

```
┌─────────────────────────────────────────────────────────┐
│  Preferences                                  ☐  ⊡  ✕   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  General                                                │
│  ┌───────────────────────────────────────────────────┐ │
│  │  ☑  Launch at login                               │ │
│  │  ☑  Show menu bar icon                            │ │
│  │  ☑  Show notifications when switching profiles    │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  Shell Integration                                      │
│  ┌───────────────────────────────────────────────────┐ │
│  │  Default Shell:  ◉ zsh   ○ bash                   │ │
│  │                                                    │ │
│  │  ☑  Auto-apply on switch                          │ │
│  │  ☑  Create activation script                      │ │
│  │                                                    │ │
│  │  Script Location:                                 │ │
│  │  ~/.envswitch/active.sh                           │ │
│  │                                                    │ │
│  │  Add to your ~/.zshrc:                            │ │
│  │  ┌──────────────────────────────────────────────┐ │ │
│  │  │ source ~/.envswitch/active.sh                │ │ │
│  │  └──────────────────────────────────────────────┘ │ │
│  │                                      [Copy Command]│ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  Storage                                                │
│  ┌───────────────────────────────────────────────────┐ │
│  │  Configuration Directory:                         │ │
│  │  ~/Library/Application Support/EnvSwitch/         │ │
│  │                                                    │ │
│  │  [Export All Configs]  [Import Configs]           │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│                                              [Close]    │
└─────────────────────────────────────────────────────────┘
```

### 4.6 导入配置对话框

```
┌─────────────────────────────────────────────────────────┐
│  Import Configuration                         ☐  ⊡  ✕   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Select Import Method:                                  │
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │  ○  From .env file                                │ │
│  │  ○  From JSON export                              │ │
│  │  ◉  From template                                 │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  Available Templates:                                   │
│  ┌───────────────────────────────────────────────────┐ │
│  │  • Node.js + Express                              │ │
│  │  • React + Vite                                   │ │
│  │  • Claude Code (Anthropic)                        │ │
│  │  • OpenAI API                                     │ │
│  │  • Supabase                                       │ │
│  │  • Vercel                                         │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│                                   [Cancel]  [Import]    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 5. 用户流程

### 5.1 首次使用流程

```
                    启动应用
                       ↓
              显示欢迎向导(可选)
                       ↓
              选择 Shell 类型
                       ↓
          是否自动创建激活脚本？
              ↓Yes        ↓No
        创建脚本并       手动配置
        提示添加到Shell
                       ↓
              创建第一个配置
                       ↓
              添加环境变量
                       ↓
                   保存
                       ↓
                   激活配置
                       ↓
              应用到环境并通知
```

### 5.2 切换配置流程

```
         点击状态栏图标
                ↓
         查看配置列表
                ↓
         选择目标配置
                ↓
      是否自动应用？
         ↓Yes    ↓No
      更新激活脚本  仅标记为激活
                ↓
       显示切换成功通知
                ↓
    (提示：在新终端或source生效)
```

### 5.3 编辑配置流程

```
         选择配置 → 点击编辑
                ↓
         进入编辑界面
                ↓
    修改名称/描述/环境变量
                ↓
            验证输入
                ↓
         ↓失败      ↓成功
    显示错误提示   保存更改
                ↓
         更新配置列表
                ↓
    如果是激活配置，重新应用
```

---

## 6. 非功能性需求

### 6.1 性能要求

- 应用启动时间 < 0.5秒
- 配置切换响应时间 < 0.2秒
- 支持至少 50 个配置无卡顿
- 内存占用 < 50MB

### 6.2 安全性要求

- 敏感信息（API Key等）支持显示隐藏
- 导出配置时提示安全警告
- 配置文件权限控制（只有用户可读写）
- 不在日志中记录敏感信息

### 6.3 可靠性要求

- 自动备份配置（保留最近 5 个版本）
- 配置文件损坏时自动恢复
- 操作失败时提供明确错误信息
- 支持撤销/重做操作（编辑时）

### 6.4 可用性要求

- 提供键盘快捷键支持
- 支持拖拽排序配置
- 支持配置搜索过滤
- 提供操作提示和帮助文档

---

## 7. 实现优先级

### P0 (MVP - 核心功能)

- ✅ 状态栏应用框架
- ✅ 配置 CRUD 功能
- ✅ 环境变量管理
- ✅ 配置激活和切换
- ✅ 生成激活脚本

### P1 (重要功能)

- 🔄 导入/导出配置
- 🔄 配置模板
- 🔄 Shell 集成验证
- 🔄 通知系统
- 🔄 配置搜索

### P2 (增强功能)

- 📋 配置备份恢复
- 📋 快捷键支持
- 📋 配置冲突检测
- 📋 使用统计
- 📋 配置分组

### P3 (未来规划)

- 💡 团队配置共享
- 💡 云端同步
- 💡 配置版本控制
- 💡 环境变量加密
- 💡 插件系统

---

## 8. 技术实现要点

### 8.1 状态栏集成

```swift
import AppKit
import SwiftUI

@main
struct EnvSwitchApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        Settings {
            EmptyView()
        }
    }
}

class AppDelegate: NSObject, NSApplicationDelegate {
    var statusItem: NSStatusItem!
    var popover = NSPopover()

    func applicationDidFinishLaunching(_ notification: Notification) {
        // 创建状态栏图标
        statusItem = NSStatusBar.system.statusItem(
            withLength: NSStatusItem.variableLength
        )

        if let button = statusItem.button {
            button.image = NSImage(
                systemSymbolName: "terminal.fill",
                accessibilityDescription: "EnvSwitch"
            )
            button.action = #selector(togglePopover)
        }

        // 配置 popover
        popover.contentViewController = NSHostingController(
            rootView: MenuView()
        )
        popover.behavior = .transient
    }
}
```

### 8.2 环境变量应用

```swift
class EnvironmentApplier {
    static func applyProfile(_ profile: EnvProfile) throws {
        let scriptPath = FileManager.default
            .homeDirectoryForCurrentUser
            .appendingPathComponent(".envswitch/active.sh")

        // 生成 shell 脚本内容
        var scriptContent = "# Auto-generated by EnvSwitch\n"
        scriptContent += "# Profile: \(profile.name)\n"
        scriptContent += "# Updated: \(Date())\n\n"

        for variable in profile.variables {
            scriptContent += "export \(variable.key)=\"\(variable.value)\"\n"
        }

        // 写入脚本文件
        try scriptContent.write(
            to: scriptPath,
            atomically: true,
            encoding: .utf8
        )

        // 设置文件权限
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o600],
            ofItemAtPath: scriptPath.path
        )
    }
}
```

### 8.3 配置存储

```swift
class ProfileStore: ObservableObject {
    @Published var profiles: [EnvProfile] = []
    private let storageURL: URL

    init() {
        let appSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!

        storageURL = appSupport
            .appendingPathComponent("EnvSwitch")
            .appendingPathComponent("profiles.json")

        loadProfiles()
    }

    func loadProfiles() {
        guard let data = try? Data(contentsOf: storageURL) else {
            return
        }

        profiles = (try? JSONDecoder().decode(
            [EnvProfile].self,
            from: data
        )) ?? []
    }

    func saveProfiles() {
        let encoder = JSONEncoder()
        encoder.outputFormatting = .prettyPrinted

        guard let data = try? encoder.encode(profiles) else {
            return
        }

        try? data.write(to: storageURL, options: .atomic)
    }
}
```

---

## 9. 测试计划

### 9.1 单元测试

- 配置 CRUD 操作
- 环境变量验证逻辑
- 脚本生成功能
- 数据持久化

### 9.2 集成测试

- Shell 集成测试（zsh/bash）
- 文件系统操作
- 配置导入导出
- 多配置切换

### 9.3 UI 测试

- 状态栏交互
- 窗口操作流程
- 快捷键响应
- 通知显示

### 9.4 兼容性测试

- macOS 13.0+
- 不同 Shell (zsh/bash)
- 不同分辨率
- Dark Mode / Light Mode

---

## 10. 发布计划

### 10.1 MVP 版本 (v1.0.0)

- 基础配置管理
- Shell 集成
- 状态栏菜单
- 配置导入导出

### 10.2 增强版本 (v1.1.0)

- 配置模板
- 快捷键支持
- 备份恢复
- 使用统计

### 10.3 专业版本 (v2.0.0)

- 团队协作
- 云端同步
- 配置加密
- 插件系统

---

## 11. 风险与对策

### 11.1 技术风险

| 风险 | 影响 | 概率 | 对策 |
|------|------|------|------|
| Shell 兼容性问题 | 高 | 中 | 充分测试主流 Shell，提供降级方案 |
| 权限问题 | 中 | 低 | 明确提示用户，提供手动配置指引 |
| 配置文件损坏 | 高 | 低 | 自动备份，版本控制 |

### 11.2 用户体验风险

| 风险 | 影响 | 概率 | 对策 |
|------|------|------|------|
| 学习成本高 | 中 | 中 | 提供向导和模板 |
| 切换不生效 | 高 | 中 | 清晰说明生效机制，提供验证工具 |
| 配置管理复杂 | 中 | 低 | 简化 UI，提供搜索和分组 |

---

## 12. 成功指标

### 12.1 产品指标

- 日活用户 > 100 (6个月内)
- 用户留存率 > 60%
- 平均配置数 > 3

### 12.2 质量指标

- Crash 率 < 0.1%
- 用户反馈评分 > 4.5/5
- Bug 修复时间 < 48小时

### 12.3 业务指标

- GitHub Stars > 500
- 社区贡献者 > 10
- 文档完整度 100%

---

## 附录 A：配置文件格式

### JSON 导出格式

```json
{
  "version": "1.0",
  "profiles": [
    {
      "id": "uuid-here",
      "name": "Development",
      "description": "Development environment",
      "variables": [
        {
          "key": "API_KEY",
          "value": "dev-123",
          "description": "API Key for development"
        }
      ],
      "isActive": true,
      "createdAt": "2024-01-01T00:00:00Z",
      "updatedAt": "2024-01-01T00:00:00Z"
    }
  ]
}
```

### .env 导入格式

```bash
# Development Configuration
API_KEY=dev-123
DB_HOST=localhost
DB_PORT=5432
NODE_ENV=development
```

---

## 附录 B：Shell 集成脚本

### Zsh 集成

```bash
# Add to ~/.zshrc
if [ -f ~/.envswitch/active.sh ]; then
    source ~/.envswitch/active.sh
fi
```

### Bash 集成

```bash
# Add to ~/.bashrc or ~/.bash_profile
if [ -f ~/.envswitch/active.sh ]; then
    source ~/.envswitch/active.sh
fi
```

---

## 附录 C：快捷键列表

| 功能 | 快捷键 |
|------|--------|
| 打开菜单 | `Cmd + Shift + E` |
| 切换到配置 1 | `Cmd + 1` |
| 切换到配置 2 | `Cmd + 2` |
| 切换到配置 3 | `Cmd + 3` |
| 新建配置 | `Cmd + N` |
| 编辑配置 | `Cmd + E` |
| 删除配置 | `Cmd + Delete` |
| 偏好设置 | `Cmd + ,` |
| 退出应用 | `Cmd + Q` |

---

**文档版本**: 1.0
**创建日期**: 2024-01-01
**最后更新**: 2024-01-01
**负责人**: Development Team
