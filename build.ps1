# build.ps1 - Compila Gutter.exe para Windows.
#
# Uso:
#   .\build.ps1            -> Gutter.exe (sin consola, optimizado)
#   .\build.ps1 -Debug     -> Gutter-debug.exe (con consola y símbolos)
#
# Requiere Go 1.26+, CGO_ENABLED=1 y un compilador de C (clang de LLVM-MinGW
# recomendado, o gcc de MinGW-w64).

param(
    [switch]$Debug,
    [switch]$ShowCompiler
)

$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "1"

function Find-InWinget([string]$exeName) {
    $roots = Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" `
        -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match "LLVM-MinGW|WinLibs" }

    foreach ($root in $roots) {
        $found = Get-ChildItem $root.FullName -Recurse -Filter $exeName `
            -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($found) { return $found.FullName }
    }
    return $null
}

function Find-Compiler {
    if ($env:CC) {
        $cmd = Get-Command $env:CC -ErrorAction SilentlyContinue
        if ($cmd) { return $cmd.Source }
        return $env:CC
    }

    # Prioridad: clang (mucho más rápido compilando Fyne que GCC 16).
    $cmd = Get-Command "clang" -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }

    $clang = Find-InWinget "clang.exe"
    if ($clang) { return $clang }

    $cmd = Get-Command "gcc" -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }

    $gcc = Find-InWinget "gcc.exe"
    if ($gcc) { return $gcc }

    throw @"
No se encontró clang ni gcc.
Instala LLVM-MinGW (recomendado) o WinLibs:
  winget install MartinStorsjo.LLVM-MinGW.UCRT
  winget install BrechtSanders.WinLibs.POSIX.UCRT
Después abre una terminal nueva y vuelve a ejecutar .\build.ps1.
"@
}

$compiler = Find-Compiler
$env:CC = $compiler
$compilerDir = Split-Path $compiler
$env:Path = "$compilerDir;$env:Path"

$compilerName = [IO.Path]::GetFileNameWithoutExtension($compiler)
if ($compilerName -match "clang") {
    $env:CXX = Join-Path $compilerDir "clang++.exe"
} else {
    $env:CXX = Join-Path $compilerDir "g++.exe"
}

Write-Host "Compilador: $compiler" -ForegroundColor DarkGray

if ($ShowCompiler) {
    return
}

if ($Debug) {
    Write-Host "Compilando Gutter-debug.exe..." -ForegroundColor Cyan
    go build -trimpath -o Gutter-debug.exe .
    $output = "Gutter-debug.exe"
} else {
    Write-Host "Compilando Gutter.exe..." -ForegroundColor Cyan
    go build -trimpath -ldflags="-s -w -H windowsgui" -o Gutter.exe .
    $output = "Gutter.exe"
}

if ($LASTEXITCODE -ne 0) {
    throw "La compilación falló con código $LASTEXITCODE."
}

$file = Get-Item $output
Write-Host ("Listo: {0} ({1:N1} MB)" -f $file.FullName, ($file.Length / 1MB)) `
    -ForegroundColor Green
