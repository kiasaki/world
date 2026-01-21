r:
	go run main.go
b:
	go build -v -o webterm main.go
i: b
	cp webterm ~/bin/webterm
ubu: i
	sudo cp systemd.service /etc/systemd/system/webterm.service
	sudo systemctl daemon-reload
	sudo systemctl enable webterm
	sudo systemctl start webterm
osx: b
	cp webterm ~/bin/webterm
	sudo cp launchd.plist /Library/LaunchDaemons/webterm.plist
	sudo launchctl unload -w /Library/LaunchDaemons/webterm.plist || true
	sudo launchctl load -w /Library/LaunchDaemons/webterm.plist
