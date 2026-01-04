#!/bin/sh
# setup-exec-wrapper.sh
# This script sets up the exec wrapper symlinks in the consumer container
# It should be run as an init container or entrypoint wrapper

set -e

WRAPPER_PATH="/.sc-bin/sc-exec"
SYMLINK_DIR="/.sc-bin"

# Common commands that should be wrapped
COMMANDS="
sh
bash
zsh
fish
ash
dash
python
python3
python2
node
npm
npx
ruby
perl
php
java
go
make
gcc
g++
clang
git
vim
vi
nano
cat
ls
cp
mv
rm
mkdir
rmdir
touch
chmod
chown
chgrp
ln
find
grep
awk
sed
head
tail
less
more
sort
uniq
wc
cut
tr
xargs
tar
gzip
gunzip
bzip2
zip
unzip
curl
wget
ssh
scp
rsync
ps
top
htop
kill
pkill
killall
env
printenv
export
echo
printf
date
cal
df
du
free
mount
umount
ping
netstat
ss
ip
ifconfig
route
iptables
hostname
uname
whoami
id
groups
su
sudo
passwd
useradd
userdel
usermod
groupadd
groupdel
groupmod
apt
apt-get
dpkg
yum
dnf
rpm
pacman
apk
pip
pip3
gem
cargo
composer
"

echo "[sc-init] Setting up exec wrapper symlinks..."

# Create symlink directory
mkdir -p "$SYMLINK_DIR"

# Create symlinks for each command
for cmd in $COMMANDS; do
    if [ ! -e "$SYMLINK_DIR/$cmd" ]; then
        ln -sf "$WRAPPER_PATH" "$SYMLINK_DIR/$cmd"
    fi
done

# Prepend our bin directory to PATH
export PATH="$SYMLINK_DIR:$PATH"

echo "[sc-init] Exec wrapper setup complete"
echo "[sc-init] PATH=$PATH"

# If arguments provided, execute them
if [ $# -gt 0 ]; then
    exec "$@"
fi
