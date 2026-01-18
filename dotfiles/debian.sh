#!/bin/bash
set -xe
sudo apt install -qq -y make cmake curl git htop mosh tmux jq neovim silversearcher-ag postgresql redis-server xclip scrot nmtui
sudo apt install -qq -y libfontconfig1-dev libxcb-xfixes0-dev libxkbcommon-dev
sudo systemctl start redis-server && sudo systemctl enable redis-server || sudo service redis-server start
sudo systemctl start postgresql && sudo systemctl enable postgresql || sudo service postgresql start
sudo -u postgres psql -c "create user $USER with superuser;" || true
sudo -u postgres psql -c "create database $USER with owner $USER;" || true

sudo cp go.ttf /usr/share/fonts/
fc-cache -f -v || true

if ! grep nocaps ~/.myenv; then
  echo "setxkbmap -option ctrl:nocaps" >> ~/.myenv
fi
