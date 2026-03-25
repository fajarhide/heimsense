#!/usr/bin/env bash
set -euo pipefail

SETTINGS_FILE="$HOME/.claude/settings.json"
ADAPTER_URL="http://localhost:8080"

# Check if settings.json exists
if [[ ! -f "$SETTINGS_FILE" ]]; then
    echo "📁 Creating ~/.claude/settings.json..."
    mkdir -p "$HOME/.claude"
    cat > "$SETTINGS_FILE" <<EOF
{
  "\$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_BASE_URL": "${ADAPTER_URL}",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}
EOF
    echo "✅ Created with ANTHROPIC_BASE_URL → ${ADAPTER_URL}"
    exit 0
fi

# Backup existing settings
cp "$SETTINGS_FILE" "${SETTINGS_FILE}.bak"
echo "💾 Backed up → ${SETTINGS_FILE}.bak"

# Check current value
CURRENT_URL=$(python3 -c "
import json, sys
with open('$SETTINGS_FILE') as f:
    data = json.load(f)
print(data.get('env', {}).get('ANTHROPIC_BASE_URL', ''))
" 2>/dev/null || echo "")

if [[ "$CURRENT_URL" == "$ADAPTER_URL" ]]; then
    echo "✅ Already configured: ANTHROPIC_BASE_URL = ${ADAPTER_URL}"
    exit 0
fi

echo "🔧 Updating ANTHROPIC_BASE_URL..."
echo "   Before: ${CURRENT_URL:-<not set>}"
echo "   After:  ${ADAPTER_URL}"

# Update using python3 (available on macOS)
python3 -c "
import json
with open('$SETTINGS_FILE') as f:
    data = json.load(f)
if 'env' not in data:
    data['env'] = {}
data['env']['ANTHROPIC_BASE_URL'] = '${ADAPTER_URL}'
with open('$SETTINGS_FILE', 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"

echo "✅ Done! Claude Code will now use the adapter at ${ADAPTER_URL}"
echo ""
echo "⚠️  Make sure the adapter is running: make run"
echo "🔄 To revert: cp ${SETTINGS_FILE}.bak ${SETTINGS_FILE}"
