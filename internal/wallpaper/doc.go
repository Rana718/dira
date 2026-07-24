// Package wallpaper sets Hyprland wallpapers via hyprpaper IPC.
//
// Flow:
//  1. zenity opens a GTK file-picker dialog
//  2. User selects an image
//  3. hyprctl hyprpaper preload <path>  — loads image into hyprpaper
//  4. hyprctl hyprpaper wallpaper <monitor>,<path>  — applies per monitor
//
// Monitors are auto-discovered via `hyprctl monitors`.
// The selected path is saved to ~/.config/dira/wallpaper so it persists.
package wallpaper
