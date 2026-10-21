# egglog-helper

`egglog-helper` 扫描 Go 项目中的 `if` 和带条件的 `for`，用 [egglog](https://github.com/egraphs-good/egglog) 查找更短的等价布尔表达式。它给出文件位置、原表达式和建议写法。规则位于 [`rules/bool.egg`](rules/bool.egg)，扫描和安全检查由 Go 程序完成。

## 构建

当前提供 Windows PowerShell 构建流程，需要 Go 1.26、Rust/Cargo 1.83 或更新版本，以及首次构建时的网络连接。项目固定使用官方 `egglog` 0.4.0；此版本可由当前环境的 Cargo 1.83 编译。构建产物放在项目的 `bin` 目录，缓存放在 `.cache`。

```powershell
cd E:\Coding\egglog-helper
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

构建后，进入任意 Go 模块运行：

```powershell
cd E:\Coding\your-go-project
E:\Coding\egglog-helper\bin\egglog-helper.exe check ./...
E:\Coding\egglog-helper\bin\egglog-helper.exe check -json ./...
```

也可以只扫描一个包，例如 `check ./internal/auth`。命令默认从工具同目录寻找 `egglog.exe`；使用 `-egglog 路径` 或环境变量 `EGGLOG_BIN` 可以指定其他位置。

退出码：`0` 表示没有建议，`1` 表示发现建议，`2` 表示参数、包加载或引擎错误。发现建议的退出码 `1` 不代表工具运行失败；在 CI 中请据此决定是否阻止提交。

## 试运行

项目自带一个小型 Go 模块：

```powershell
cd E:\Coding\egglog-helper\examples\sample
..\..\bin\egglog-helper.exe check ./...
..\..\bin\egglog-helper.exe check -json ./...
```

示例中的 `enabled && (admin || !admin)` 应被建议为 `enabled`。带字段访问的 `user != nil && user.IsAdmin` 会被跳过，因为其中的短路求值关系需要保留。

## 当前支持范围

工具处理最多 8 个独立布尔命题的条件，支持局部变量或参数、常量、`!`、`&&`、`||`，以及局部变量与常量之间的简单 `==`、`!=` 比较。它扫描当前构建配置下编译的 Go 文件，并跳过标记为生成代码的文件。

函数调用、字段访问、下标、解引用、通道接收和类型断言等表达式会被跳过。每条建议都经过额外的真值表校验。结果是供开发者审阅的建议，工具不会修改目标源码。

## 开发

运行 `go test ./...` 和 `go vet ./...`。集成测试在项目的 `.tools/bin` 找 `egglog.exe`，也可设置 `EGGLOG_BIN`。在没有引擎时，直接测试表达式筛选的用例仍可运行，而真实引擎的用例会跳过。

修改规则时，请在 `rules/bool.egg` 添加等价变换，并用真实 egglog 运行 `go test ./internal/engine ./internal/check ./cmd/egglog-helper`。Go 端会独立拒绝任何不等价的提取结果。
