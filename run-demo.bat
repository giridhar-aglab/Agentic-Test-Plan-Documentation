@echo off
REM Level-by-level check that everything is configured. Run from the project root.
REM Each level proves what the next depends on, so a failure tells you where to look.
setlocal

echo === Level 0: build and tests, no credentials needed ===
go build ./... || goto :fail
go test ./... || goto :fail
echo.

echo === Level 0: pipeline against the bundled fixture ===
go run ./cmd/testplan -source testdata\fixtures\paymentsvc -name paymentsvc -out level0.md || goto :fail
echo wrote level0.md ^(placeholder scenarios if no API key^)
echo.

if "%ANTHROPIC_API_KEY%"=="" (
  echo === Level 1 SKIPPED: ANTHROPIC_API_KEY is not set ===
  echo     set ANTHROPIC_API_KEY=sk-ant-...
  echo.
) else (
  echo === Level 1: real scenarios on the fixture ===
  go run ./cmd/testplan -source testdata\fixtures\paymentsvc -name paymentsvc -v -out level1.md || goto :fail
  echo wrote level1.md
  echo.
)

if "%GITHUB_MCP_COMMAND%"=="" (
  echo === Level 3 SKIPPED: GITHUB_MCP_COMMAND is not set ===
  echo     go install github.com/github/github-mcp-server/cmd/github-mcp-server@latest
  echo     set GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...
  echo     set GITHUB_MCP_COMMAND=github-mcp-server stdio
  echo.
) else (
  echo === Level 3: MCP preflight ===
  go run ./cmd/testplan -github https://github.com/gin-gonic/gin -mcp-check || goto :fail
  echo.
)

echo === All configured levels passed ===
exit /b 0

:fail
echo.
echo === FAILED. See SETUP.md troubleshooting. ===
exit /b 1
