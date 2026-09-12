# assets/

`SumatraPDF.exe` in this directory is embedded into the compiled binary via
`go:embed` and extracted at runtime to drive silent, headless printing.

**Before building, replace the placeholder file with the real portable
64-bit SumatraPDF executable** (download from https://www.sumatrapdfreader.org/download-free-pdf-viewer,
"32-bit/64-bit portable" build — do NOT use the installer build). Keep the
filename exactly `SumatraPDF.exe`.

The placeholder committed here is a 0-byte stub so the project compiles;
it will fail at runtime with an "exec format error" until you swap in the
real binary.
