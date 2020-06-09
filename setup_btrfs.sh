#!/bin/sh

truncate -s256M btrfs.img

ld=$(sudo losetup --show --find btrfs.img)

sudo mkfs -t btrfs "$ld"

mkdir /tmp/btrfs
sudo mount $ld /tmp/btrfs

sudo chown $USER:$USER /tmp/btrfs
