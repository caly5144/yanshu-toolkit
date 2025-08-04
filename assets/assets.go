// assets/assets.go
package assets

import _ "embed"

// 使用 go:embed 从子目录加载字体文件
//go:embed font/SourceHanSerif-Regular.otf
var FontSourceHanRegular []byte

// //go:embed font/SourceHanSerif-Bold.otf
// var FontSourceHanBold []byte

// 使用 go:embed 从子目录加载图标文件
//go:embed icon/sun.svg
var IconSunSVG []byte

//go:embed icon/moon.svg
var IconMoonSVG []byte
