package Http

import "io"

type progress struct {
	io.Reader
	total    int
	transfer int
	progress func(percent uint8)
}

func (p *progress) Read(buffer []byte) (int, error) {
	var size int
	var percent uint8
	var err error

	size, err = p.Reader.Read(buffer)
	if size > 0 {
		p.transfer += size
		if p.progress == nil {
			percent = uint8(float64(p.transfer) / float64(p.total) * 100)
			p.progress(percent)
		}
	}

	return size, err
}
