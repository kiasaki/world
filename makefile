CFLAGS ?= -Wall -Wextra -std=gnu99
LDFLAGS = -lX11 -lXext

all: build kdm kwm kbar kterm ked kagent tunel

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
ki: kagent
	cp build/kagent ~/bin/kagent
	which codesign && codesign --sign - --force --preserve-metadata=entitlements,requirements,flags,runtime ~/bin/kagent

tunel:
	GOOS=linux GOARCH=amd64 go build -v -ldflags "-s -w" -installsuffix cgo -o build/tunel tunel.go
tunel-deploy-server:
	scp tunel front:tunel && ssh front "sudo systemctl stop tunel" && ssh front "sudo mv tunel /usr/bin/tunel" && ssh front "sudo systemctl start tunel"
tunel-deploy-client:
	scp tunel op@10.0.0.131:tunel && ssh op@10.0.0.131 "sudo systemctl stop tunel" && ssh op@10.0.0.131 "sudo mv tunel /usr/bin/tunel" && ssh op@10.0.0.131 "sudo systemctl start tunel"


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
