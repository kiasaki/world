#!/bin/bash
set -xe
rm -f ~/.bashrc && cp bashrc ~/.bashrc
rm -f ~/.psqlrc && cp psqlrc ~/.psqlrc
rm -f ~/.sqliterc && cp sqliterc ~/.sqliterc
rm -f ~/.tmux.conf && cp tmux.conf ~/.tmux.conf
rm -f ~/.gitconfig && cp gitconfig ~/.gitconfig
mkdir -p ~/.config/nvim/colors ~/.config/nvim/syntax ~/.config/nvim/autoload
rm -f ~/.config/nvim/init.vim && cp vimrc ~/.config/nvim/init.vim
rm -f ~/.config/nvim/colors/u.vim && cp vimu.vim ~/.config/nvim/colors/u.vim
rm -f ~/.config/nvim/syntax/go.vim && cp vimgo.vim ~/.config/nvim/syntax/go.vim
rm -f ~/.config/nvim/autoload/plug.vim && cp vimplug.vim ~/.config/nvim/autoload/plug.vim
touch ~/.myenv ~/.hushlogin

if [ ! -f $HOME/goroot/bin/go ]; then
  wget -O go.tar.gz https://go.dev/dl/go1.25.5.linux-amd64.tar.gz
  tar -xzf go.tar.gz
  mv go ~/goroot
  rm go.tar.gz
  export GOROOT=~/goroot GOPATH=~/gopath GOBIN=~/bin
  $GOROOT/bin/go install golang.org/x/tools/cmd/goimports@latest
fi

if [ ! -d "$HOME/.rustup" ]; then
  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | bash -s -- -y
fi

if [ ! -d "$HOME/n" ]; then
  curl -L http://git.io/n-install | bash -s -- -n -y
  export PATH="$PATH:$HOME/n/bin"
  $HOME/n/bin/npm i -g eslint
  $HOME/n/bin/npm i -g prettier
fi
