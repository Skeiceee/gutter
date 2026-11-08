//go:build windows

package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// platformOCR usa el motor OCR integrado de Windows 10/11 a través de
// PowerShell y WinRT. No requiere instalar Tesseract ni compilar con cgo.
func platformOCR(ctx context.Context, imagePath string, source languageOption) ([]ocrLine, error) {
	langTag := source.Windows
	if langTag == "" {
		langTag = "en-US"
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(
		ctx,
		powershellExecutable(),
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", windowsOCRPowerShell,
	)
	cmd.Env = withEnv(cmd.Environ(),
		"GUTTER_OCR_IMAGE", imagePath,
		"GUTTER_OCR_LANG", langTag,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	hideConsole(cmd)

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("OCR de Windows: %s", detail)
	}

	var rows []struct {
		Text string  `json:"text"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		W    float64 `json:"w"`
		H    float64 `json:"h"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &rows); err != nil {
		return nil, fmt.Errorf("OCR de Windows devolvió una respuesta inválida: %w", err)
	}

	lines := make([]ocrLine, 0, len(rows))
	for _, row := range rows {
		text := strings.TrimSpace(row.Text)
		rect := rectFromFloats(row.X, row.Y, row.W, row.H)
		if text == "" || rect.Dx() <= 0 || rect.Dy() <= 0 {
			continue
		}
		lines = append(lines, ocrLine{Text: text, Rect: rect})
	}
	return lines, nil
}

func powershellExecutable() string {
	if path, err := exec.LookPath("powershell.exe"); err == nil {
		return path
	}
	if path, err := exec.LookPath("pwsh.exe"); err == nil {
		return path
	}
	return "powershell.exe"
}

// windowsOCRPowerShell es autónomo: recibe la imagen y el idioma por variables
// de entorno y devuelve un array JSON de líneas con su bounding box.
//
// Se evita el uso de backticks para poder guardarlo como cadena raw de Go.
const windowsOCRPowerShell = `
$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$OutputEncoding = [System.Text.Encoding]::UTF8
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Media.Ocr.OcrEngine,Windows.Foundation,ContentType=WindowsRuntime]
$null = [Windows.Graphics.Imaging.BitmapDecoder,Windows.Foundation,ContentType=WindowsRuntime]
$null = [Windows.Storage.StorageFile,Windows.Foundation,ContentType=WindowsRuntime]
$null = [Windows.Storage.Streams.IRandomAccessStream,Windows.Foundation,ContentType=WindowsRuntime]
$null = [Windows.Globalization.Language,Windows.Foundation,ContentType=WindowsRuntime]

$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
    $_.Name -eq "AsTask" -and
    $_.GetParameters().Count -eq 1 -and
    $_.GetParameters()[0].ParameterType.Name -like "IAsyncOperation*"
})[0]

function Await($WinRtTask, $ResultType) {
    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
    $netTask = $asTask.Invoke($null, @($WinRtTask))
    $netTask.Wait(-1) | Out-Null
    $netTask.Result
}

$imagePath = $env:GUTTER_OCR_IMAGE
$langTag = $env:GUTTER_OCR_LANG
if (-not (Test-Path -LiteralPath $imagePath)) {
    throw "No existe la imagen temporal de OCR: $imagePath"
}

$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($imagePath)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
$decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
$bitmap = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])

if ($langTag) {
    $language = New-Object Windows.Globalization.Language($langTag)
    $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($language)
} else {
    $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
}
if (-not $engine) {
    $available = ([Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages | ForEach-Object { $_.LanguageTag }) -join ", "
    throw "El idioma OCR '$langTag' no está instalado. Disponibles: $available"
}

$result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
$rows = @()
foreach ($line in $result.Lines) {
    $words = @($line.Words)
    if ($words.Count -eq 0) { continue }

    $minX = [double]::MaxValue
    $minY = [double]::MaxValue
    $maxX = [double]::MinValue
    $maxY = [double]::MinValue
    foreach ($word in $words) {
        $r = $word.BoundingRect
        if ($r.X -lt $minX) { $minX = $r.X }
        if ($r.Y -lt $minY) { $minY = $r.Y }
        if (($r.X + $r.Width) -gt $maxX) { $maxX = $r.X + $r.Width }
        if (($r.Y + $r.Height) -gt $maxY) { $maxY = $r.Y + $r.Height }
    }

    $rows += [pscustomobject]@{
        text = $line.Text
        x = [double]$minX
        y = [double]$minY
        w = [double]($maxX - $minX)
        h = [double]($maxY - $minY)
    }
}

if ($rows.Count -eq 0) {
    "[]"
} else {
    ConvertTo-Json -InputObject @($rows) -Compress -Depth 3
}
`
