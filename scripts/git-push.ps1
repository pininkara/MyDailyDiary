param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Version
)

$ErrorActionPreference = "Stop"

# 支持传入 1.1.0 或 v1.1.0
if ($Version.StartsWith("v")) {
    $Tag = $Version
} else {
    $Tag = "v$Version"
}

# 获取当前分支
$Branch = git branch --show-current

if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($Branch)) {
    Write-Error "无法获取当前 Git 分支"
    exit 1
}

Write-Host "Branch: $Branch"
Write-Host "Tag:    $Tag"
Write-Host ""

# 检查 tag 是否已存在
$ExistingTag = git tag --list $Tag

if ($ExistingTag -eq $Tag) {
    Write-Error "Tag '$Tag' 已经存在"
    exit 1
}

function Invoke-GitPush {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Remote,

        [Parameter(Mandatory = $true)]
        [string]$Ref,

        [int]$RetryCount = 0
    )

    for ($Attempt = 0; $Attempt -le $RetryCount; $Attempt++) {
        if ($Attempt -gt 0) {
            Write-Host ""
            Write-Warning "推送 $Remote $Ref 失败，正在重试 ($Attempt/$RetryCount)..."
            Start-Sleep -Seconds 1
        }

        git push $Remote $Ref

        if ($LASTEXITCODE -eq 0) {
            return
        }
    }

    Write-Error "git push $Remote $Ref 最终失败"
    exit 1
}

# ------------------------------------------------------------
# 创建 tag
# ------------------------------------------------------------

Write-Host "==> 创建 Tag $Tag"

git tag $Tag

if ($LASTEXITCODE -ne 0) {
    Write-Error "创建 Tag $Tag 失败"
    exit 1
}

# ------------------------------------------------------------
# GitHub
# ------------------------------------------------------------

Write-Host ""
Write-Host "==> 推送分支到 github"

Invoke-GitPush `
    -Remote "github" `
    -Ref $Branch

Write-Host ""
Write-Host "==> 推送 Tag 到 github"

Invoke-GitPush `
    -Remote "github" `
    -Ref $Tag

# ------------------------------------------------------------
# Gitea
# 第一次认证已知可能失败，因此允许自动重试 1 次
# ------------------------------------------------------------

Write-Host ""
Write-Host "==> 推送分支到 Gitea"

Invoke-GitPush `
    -Remote "Gitea" `
    -Ref $Branch `
    -RetryCount 1

Write-Host ""
Write-Host "==> 推送 Tag 到 Gitea"

Invoke-GitPush `
    -Remote "Gitea" `
    -Ref $Tag `
    -RetryCount 1

# ------------------------------------------------------------

Write-Host ""
Write-Host "========================================"
Write-Host "发布完成"
Write-Host "Branch : $Branch"
Write-Host "Tag    : $Tag"
Write-Host "========================================"