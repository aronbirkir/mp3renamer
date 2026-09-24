package main

import _ "embed"

//go:generate go run ./tools/genicon icon.png

//go:embed icon.png
var iconPNG []byte
