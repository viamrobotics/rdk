{ pkgs ? import (fetchTarball
  "https://github.com/NixOS/nixpkgs/archive/54ce16370bc03831b7d225b64a1ff82b383e0242.tar.gz")
  { } }:

pkgs.mkShell {
  buildInputs =
    [ pkgs.which pkgs.htop pkgs.go pkgs.nodejs pkgs.pkg-config pkgs.libvpx pkgs.x264 pkgs.libopus ]
    ++ pkgs.lib.optionals pkgs.stdenv.isDarwin [
      pkgs.darwin.apple_sdk.frameworks.AVFoundation
      pkgs.darwin.apple_sdk.frameworks.CoreMedia
    ];

  outputs = [ "out" "doc" "dev" ];
}
