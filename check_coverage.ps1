$ErrorActionPreference = "Stop"

Write-Host "=== Генерация отчета о покрытии кода ===" -ForegroundColor Cyan
go test -cover -coverprofile=coverage.out ./pkg/event
if ($LASTEXITCODE -ne 0) { 
    Write-Host "Тесты провалены. Генерация отчета из последнего успешного запуска..." -ForegroundColor Yellow 
    exit 1 
}

Write-Host "`n=== Отчет о покрытии по функциям ===" -ForegroundColor Cyan
go tool cover -func=coverage.out

Write-Host "`n=== Анализ непокрытых строк ===" -ForegroundColor Yellow
go tool cover -func=coverage.out | Select-String "0.0%" | ForEach-Object { Write-Host $_ -ForegroundColor Red }

Write-Host "`n=== Генерация HTML отчета ===" -ForegroundColor Cyan
go tool cover -html=coverage.out -o coverage.html
if ($LASTEXITCODE -eq 0) {
    Write-Host "HTML отчет создан: coverage.html" -ForegroundColor Green
}

