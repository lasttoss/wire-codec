package codec_test

import (
	"bytes"
	"io"
)

func stringsReader(s string) io.Reader { return bytes.NewReader([]byte(s)) }
