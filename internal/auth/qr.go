package auth

import (
	"fmt"
	"os"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// QRHandler handles QR code display and pairing flow.
type QRHandler struct {
	log waLog.Logger
}

// NewQRHandler creates a new QRHandler.
func NewQRHandler(log waLog.Logger) *QRHandler {
	return &QRHandler{log: log.Sub("QR")}
}

// HandleQRChannel processes QR channel items and displays QR codes.
func (h *QRHandler) HandleQRChannel(qrChan <-chan whatsmeow.QRChannelItem) error {
	for item := range qrChan {
		switch item.Event {
		case "code":
			h.log.Infof("Scan the QR code below with WhatsApp (Linked Devices)")
			h.displayQR(item.Code)
		case "timeout":
			h.log.Warnf("QR code timeout - please restart to get a new QR code")
			return fmt.Errorf("QR code timeout")
		case "success":
			h.log.Infof("Successfully paired!")
			return nil
		case "error":
			h.log.Errorf("QR error: %v", item.Error)
			return item.Error
		}
	}
	return nil
}

// displayQR displays a QR code in the terminal.
func (h *QRHandler) displayQR(code string) {
	// Try to display in terminal using ASCII
	qr, err := qrcode.New(code, qrcode.Medium)
	if err != nil {
		h.log.Errorf("Failed to generate QR code: %v", err)
		fmt.Println("QR Code content:", code)
		return
	}

	// Print as ASCII art
	fmt.Println()
	fmt.Println(qr.ToSmallString(false))
	fmt.Println()
}

// SaveQRToFile saves the QR code to a file.
func (h *QRHandler) SaveQRToFile(code, filepath string) error {
	err := qrcode.WriteFile(code, qrcode.Medium, 256, filepath)
	if err != nil {
		return fmt.Errorf("failed to save QR code: %w", err)
	}
	h.log.Infof("QR code saved to %s", filepath)
	return nil
}

// ClearScreen clears the terminal screen.
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
	os.Stdout.Sync()
}
