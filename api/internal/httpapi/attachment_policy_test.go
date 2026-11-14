package httpapi

import "testing"

func TestAttachmentNamesMatchContentAndRejectLaunchableFiles(t *testing.T) {
	for _, name := range []string{"report.exe", "launch.BAT", "invoice.pdf.exe", "shell.ps1", "shortcut.lnk", "site.html", "image.svg"} {
		if safeAttachmentName(name, "text/plain; charset=utf-8") {
			t.Errorf("accepted launchable file %s", name)
		}
	}
	for _, value := range [][2]string{{"fake.pdf", "text/plain; charset=utf-8"}, {"fake.mp4", "application/pdf"}, {"fake.txt", "application/pdf"}, {"fake.docx", "text/plain; charset=utf-8"}} {
		if safeAttachmentName(value[0], value[1]) {
			t.Errorf("mismatched extension %v", value)
		}
	}
	for _, value := range [][2]string{{"report.pdf", "application/pdf"}, {"notes.txt", "text/plain; charset=utf-8"}, {"data.csv", "text/plain; charset=utf-8"}, {"sheet.xlsx", "application/zip"}, {"video.mp4", "video/mp4"}} {
		if !safeAttachmentName(value[0], value[1]) {
			t.Errorf("rejected supported file %v", value)
		}
	}
}
