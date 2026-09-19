package violating

import "image/png"

func UsePNG() {
	_ = png.Decode
}
