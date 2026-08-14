package container

import (
	"fmt"
	"os"
	"os/exec"
)

func editFileScript(rt, id, name string) string {
	editor := getEditor()
	return fmt.Sprintf(`
set -e
echo ""
echo "  ═══ dira: edit container file ═══"
echo "  Container: %s (%s)"
echo ""
echo "  File tree (showing config-relevant paths):"
echo "  ─────────────────────────────────────────"
%s exec %s find /etc /app /data /config /opt 2>/dev/null | head -60 || true
echo ""
echo "  ─────────────────────────────────────────"
echo ""
printf "  Enter file path to edit (or 'q' to cancel): "
read filepath
if [ "$filepath" = "q" ] || [ -z "$filepath" ]; then
    echo "  Cancelled."
    exit 0
fi
tmpfile=$(mktemp /tmp/dira-edit-XXXXXX)
%s cp %s:"$filepath" "$tmpfile" 2>/dev/null
if [ $? -ne 0 ]; then
    echo "  Error: cannot read $filepath from container"
    rm -f "$tmpfile"
    exit 1
fi
%s "$tmpfile"
%s cp "$tmpfile" %s:"$filepath"
if [ $? -eq 0 ]; then
    echo ""
    echo "  ✓ File updated: $filepath"
    printf "  Restart container to apply? [y/N]: "
    read restart
    if [ "$restart" = "y" ] || [ "$restart" = "Y" ]; then
        %s restart %s
        echo "  ✓ Container restarted."
    fi
else
    echo "  ✗ Failed to copy file back."
fi
rm -f "$tmpfile"
`, name, id[:12], rt, id, rt, id, editor, rt, id, rt, id)
}

func getEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	for _, e := range []string{"vim", "nvim", "nano", "vi"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return "vi"
}
