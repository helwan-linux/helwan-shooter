# Maintainer: Saeed Badreldin <helwanlinux@gmail.com>
# Part of Helwan Linux

pkgname=shooter
pkgver=0.1.0
pkgrel=3
pkgdesc="Mouse-aim arcade target-shooting game"
arch=('x86_64')
url="https://github.com/helwan-linux/helwan-shooter"
license=('LicenseRef-HSAL-1.0')
depends=('glibc' 'hicolor-icon-theme')
makedepends=('go')
source=("$pkgname::git+$url.git")
sha256sums=('SKIP')
options=('!debug')

build() {
    cd "$srcdir/$pkgname"

    export CGO_CPPFLAGS="${CPPFLAGS}"
    export CGO_CFLAGS="${CFLAGS}"
    export CGO_CXXFLAGS="${CXXFLAGS}"
    export CGO_LDFLAGS="${LDFLAGS}"
    export GOPATH="$srcdir/go"
    export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"

    go build \
        -ldflags="-linkmode=external" \
        -o shooter \
        ./cmd/helwan-shooter
}

package() {
    cd "$srcdir/$pkgname"

    install -Dm755 shooter \
        "$pkgdir/usr/bin/shooter"

    install -Dm644 assets/icon.png \
        "$pkgdir/usr/share/icons/hicolor/256x256/apps/shooter.png"

    install -Dm644 assets/helwan-shooter.png \
        "$pkgdir/usr/share/pixmaps/helwan-shooter.png"

    install -Dm644 README.md \
        "$pkgdir/usr/share/doc/shooter/README.md"

    install -d "$pkgdir/usr/share/applications"

    cat > "$pkgdir/usr/share/applications/helwan-shooter.desktop" <<'EOF'
[Desktop Entry]
Version=1.0
Type=Application
Name=Helwan Shooter
Comment=Play Helwan Shooter
Exec=/usr/bin/shooter
TryExec=/usr/bin/shooter
Icon=shooter
Terminal=false
Categories=Game;ArcadeGame;
StartupNotify=true
EOF

    chmod 644 "$pkgdir/usr/share/applications/helwan-shooter.desktop"
}
