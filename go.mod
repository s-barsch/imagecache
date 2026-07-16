module g.rg-s.com/imgcache

go 1.25.1

require (
	github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd
	gopkg.in/gographics/imagick.v3 v3.7.2
)

require golang.org/x/sync v0.19.0

replace g.rg-s.com/imgcache/cache => ./cache
