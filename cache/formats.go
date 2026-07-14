package cache

type Format uint8

const (
	JPEG Format = iota + 1
	WEBP
	PNG
	AVIF
)

func (f Format) String() string {
	switch f {
	case JPEG:
		return "jpg"
	case WEBP:
		return "webp"
	case PNG:
		return "png"
	case AVIF:
		return "avif"
	}
	panic("invalid format")
}

func (f Format) Ext() string {
	return "." + f.String()
}

func AllFormats() []Format {
	return []Format{JPEG, WEBP, PNG, AVIF}
}

func CacheFormats() []Format {
	return []Format{JPEG, AVIF}
}

func (f Format) MagickFormat() string {
	switch f {
	case JPEG:
		return "JPEG"
	case AVIF:
		return "AVIF"
	}
	panic("unsupported cache format")
}
