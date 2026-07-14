package cache

type Format uint8

const (
	JPEG Format = iota + 1
	WEBP
	AVIF
)

func (f Format) String() string {
	switch f {
	case JPEG:
		return "jpg"
	case WEBP:
		return "webp"
	case AVIF:
		return "avif"
	}
	panic("invalid format")
}

func (f Format) Ext() string {
	return "." + f.String()
}

func AllFormats() []Format {
	return []Format{JPEG, WEBP, AVIF}
}
