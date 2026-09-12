// Package docprocessor defines the extensibility seam between "a file was
// downloaded" and "a file is ready to hand to the printer engine". Phase 1
// only ever receives PDFs, so DirectPdfProcessor is a no-op passthrough.
// Phase 2 (legal DOCX automation) can inject a LibreOfficeProcessor that
// converts DOCX -> PDF headlessly before printing, without touching any
// other part of the pipeline.
package docprocessor

// DocumentProcessor converts an input file into a print-ready file,
// returning the path to the (possibly new) output file.
type DocumentProcessor interface {
	// Process takes the path to a downloaded input file and returns the
	// path to a file that the printer engine can send directly to
	// SumatraPDF. Implementations that don't need conversion may simply
	// return the input path unchanged.
	Process(inputPath string) (outputPath string, err error)
}

// DirectPdfProcessor is the Phase 1 implementation: every job that reaches
// the desktop agent is already a PDF, so no conversion is required.
type DirectPdfProcessor struct{}

// NewDirectPdfProcessor returns a DocumentProcessor that passes PDFs through
// unmodified.
func NewDirectPdfProcessor() *DirectPdfProcessor {
	return &DirectPdfProcessor{}
}

// Process implements DocumentProcessor by returning the input path as-is.
func (p *DirectPdfProcessor) Process(inputPath string) (string, error) {
	return inputPath, nil
}

// Future Phase 2 sketch (not implemented here — left as documentation of
// the intended seam):
//
//	type LibreOfficeProcessor struct {
//	    SofficePath string
//	}
//
//	func (p *LibreOfficeProcessor) Process(inputPath string) (string, error) {
//	    // shell out to `soffice --headless --convert-to pdf inputPath`,
//	    // return the resulting .pdf path.
//	}
