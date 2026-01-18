CFLAGS ?= -Wall -Wextra -std=gnu99
LDFLAGS = -lX11 -lXext

all: build kdm kwm kbar kterm ked kagent

clean:
	rm -f build

build:
	mkdir build

kdm: dm.c
	$(CC) dm.c -o build/$@ $(CFLAGS) $(LDFLAGS) -lpam -lpam_misc

kwm: wm.c
	$(CC) wm.c -o build/$@ $(CFLAGS) $(LDFLAGS) -lXinerama

kbar: bar.c
	$(CC) bar.c -o build/$@ $(CFLAGS) $(LDFLAGS)

kterm: term.c
	$(CC) term.c tsm/tsm-*.c -o build/$@ $(CFLAGS) $(LDFLAGS)

ked:
	go build -o build/$@ ed.go

kagent:
	go build -o build/$@ agent.go


install: all
	install -m 755 build/kdm /usr/bin/
	install -m 755 build/kwm /usr/bin/
	install -m 755 build/kbar /usr/bin/
	install -m 644 dm.pam /etc/pam.d/kdm
	cp dm.service /etc/systemd/system/display-manager.service
	chown root:root /etc/systemd/system/display-manager.service
	install -m 755 build/kterm ~/bin/
	install -m 755 build/ked ~/bin/

font:
	xxd -i chicago12.uf2 > chicago12.h
	sed -i s/chicago12_uf2/chicago/g chicago12.h

x:
	Xephyr -ac -br -noreset -screen 1024x768 :1

bg-colors:
	convert xc:white xc:black xc:'#c0c0c0' xc:'#ffd2f9' +append /tmp/colors.gif

bg-dither:
	convert $(BG) -dither FloydSteinberg -remap /tmp/colors.gif -negate dotfiles/bg.png

bg:
	feh --bg-fill dotfiles/bg.png
