package stickynotes

const (
	PODir             = "po"
	MODir             = "locale"
	LocaleDomain      = "indicator-stickynotes"
	SettingsFile      = "~/.config/indicator-stickynotes"
	DebugSettingsFile = "~/.stickynotes"
)

var FallbackProperties = map[string]interface{}{
	"bgcolor_hsv": []float64{48.0 / 360, 1, 1},
	"textcolor":   []float64{32.0 / 255, 32.0 / 255, 32.0 / 255},
	"font":        "",
	"shadow":      60,
}
