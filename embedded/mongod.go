package embedded

import _ "embed"

// Встроенные бинарники MongoDB для разных платформ
// Бинарники взяты из официальных архивов MongoDB (лицензия SSPL)
// Для production рекомендуется использовать внешний MongoDB

//go:embed mongod-windows-amd64.exe
var MongodWindows []byte

//go:embed mongod-linux-amd64
var MongodLinux []byte

//go:embed mongod-darwin-amd64
var MongodDarwin []byte
