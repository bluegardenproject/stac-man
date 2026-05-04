#!/bin/bash
#
# Removes stac-man cleanly: deletes the binary, removes the PATH line
# from rc files written by install.sh, and (optionally) drops the user
# config directory. Per-repo stack metadata lives in each repo's
# .git/config and is intentionally NOT touched — those repos are still
# valid after uninstall.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/uninstall.sh | bash
#
#   ./uninstall.sh                # interactive (asks before deleting config)
#   ./uninstall.sh --keep-config  # never touch ~/.config/stac-man
#   ./uninstall.sh --purge        # also delete ~/.config/stac-man without asking

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

INSTALL_DIR="$HOME/.stac-man"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/stac-man"
BINARY_NAME="sm"

MODE="ask"
for arg in "$@"; do
    case "$arg" in
        --keep-config) MODE="keep" ;;
        --purge) MODE="purge" ;;
        -h|--help)
            sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown flag: $arg${NC}" >&2
            exit 2
            ;;
    esac
done

echo -e "${BOLD}${BLUE}stac-man Uninstaller${NC}"
echo

# 1. Binary
if [ -d "$INSTALL_DIR" ]; then
    echo -e "${BLUE}Removing binary directory:${NC} $INSTALL_DIR"
    rm -rf "$INSTALL_DIR"
    echo -e "${GREEN}  removed${NC}"
else
    echo -e "${YELLOW}No binary directory at $INSTALL_DIR (already gone?)${NC}"
fi

# 2. PATH lines. We only strip lines that reference $HOME/.stac-man
# AND sit beneath our marker comment, so a hand-edited rc isn't
# clobbered.
strip_rc() {
    local rc="$1"
    [ -f "$rc" ] || return 0

    # Match the marker line and the immediately following PATH line.
    # Portable sed: write to a tempfile then move back.
    local tmp
    tmp="$(mktemp)"
    awk -v dir="$INSTALL_DIR" '
        /^# stac-man \(auto-added by install.sh\)$/ { skip_next = 1; next }
        skip_next == 1 && index($0, dir) > 0 { skip_next = 0; next }
        { skip_next = 0; print }
    ' "$rc" > "$tmp"

    if ! cmp -s "$rc" "$tmp"; then
        mv "$tmp" "$rc"
        echo -e "${GREEN}  cleaned $rc${NC}"
    else
        rm -f "$tmp"
        # Fallback: any remaining line that mentions our install dir
        # (e.g. a hand-edited rc that doesn't have our marker). Tell
        # the user instead of silently deleting.
        if grep -Fq "$INSTALL_DIR" "$rc"; then
            echo -e "${YELLOW}  $rc still references $INSTALL_DIR but has no install.sh marker.${NC}"
            echo -e "${YELLOW}  Remove the line manually if you want a fully clean uninstall.${NC}"
        fi
    fi
}

echo -e "${BLUE}Cleaning shell rc files...${NC}"
strip_rc "$HOME/.zshrc"
strip_rc "${ZDOTDIR:-$HOME}/.zshrc"
strip_rc "$HOME/.bashrc"
strip_rc "$HOME/.bash_profile"
strip_rc "$HOME/.profile"

# fish: the conf.d snippet is ours alone, so we just delete the file.
FISH_CONF="${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/stac-man.fish"
if [ -f "$FISH_CONF" ]; then
    rm -f "$FISH_CONF"
    echo -e "${GREEN}  removed $FISH_CONF${NC}"
fi

# 3. User config — opt-in.
if [ -d "$CONFIG_DIR" ]; then
    case "$MODE" in
        keep)
            echo -e "${YELLOW}Keeping config directory:${NC} $CONFIG_DIR"
            ;;
        purge)
            rm -rf "$CONFIG_DIR"
            echo -e "${GREEN}Removed config directory:${NC} $CONFIG_DIR"
            ;;
        ask)
            # Skip the prompt when piped from curl — we have no tty.
            if [ -t 0 ]; then
                echo
                printf "Also remove user config at %s? [y/N] " "$CONFIG_DIR"
                read -r reply
                case "$reply" in
                    y|Y|yes|YES)
                        rm -rf "$CONFIG_DIR"
                        echo -e "${GREEN}  removed${NC}"
                        ;;
                    *)
                        echo -e "${YELLOW}  kept${NC}"
                        ;;
                esac
            else
                echo -e "${YELLOW}Keeping config directory (run with --purge to remove): $CONFIG_DIR${NC}"
            fi
            ;;
    esac
fi

echo
echo -e "${BOLD}${GREEN}Uninstall complete.${NC}"
echo
echo -e "${YELLOW}Per-repo stack metadata in each repo's .git/config was left in place;${NC}"
echo -e "${YELLOW}it has no effect once sm is gone, and is reused if you reinstall.${NC}"
echo -e "${YELLOW}Open a new shell to drop $INSTALL_DIR from PATH.${NC}"
echo
