@echo off
setlocal enabledelayedexpansion

set SHARD_NUM=2
set NODE_NUM=4

rd /s /q .\exp\ 2>nul
mkdir .\exp\

set GOFLAGS=-modcacherw

echo Downloading Go modules...
go mod download
if %errorlevel% neq 0 exit /b %errorlevel%

echo Building project...
go build ./...
if %errorlevel% neq 0 exit /b %errorlevel%

:: 启动 supervisor
start "Supervisor" cmd /k "go run cmd\supervisor\main.go -shard_id=0x7fffffff -node_id=0"

:: 启动 consensus nodes: 外层 shard_id = i, 内层 node_id = j
for /l %%i in (0,1,%SHARD_NUM%) do (
    if %%i lss %SHARD_NUM% (
        for /l %%j in (0,1,%NODE_NUM%) do (
            if %%j lss %NODE_NUM% (
                start "ConsensusNode-%%i-%%j" cmd /k "go run cmd\consensusnode\main.go -shard_id=%%i -node_id=%%j"
            )
        )
    )
)
