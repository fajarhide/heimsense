#!/usr/bin/env bash
set -euo pipefail

SETTINGS_FILE="$HOME/.claude/settings.json"
# Variables from environment (passed via Makefile or sourced)
ADAPTER_URL="${ADAPTER_URL:-http://localhost:8080}"
CUSTOM_MODEL_ID="${ANTHROPIC_CUSTOM_MODEL_OPTION:-glm-5.1}"
CUSTOM_MODEL_NAME="${ANTHROPIC_CUSTOM_MODEL_OPTION_NAME:-Heimsense Custom Model}"
CUSTOM_MODEL_DESC="${ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION:-Custom model via Heimsense adapter}"

# Check if settings.json exists
if [[ ! -f "$SETTINGS_FILE" ]]; then
    echo "📁 Creating ~/.claude/settings.json..."
    mkdir -p "$HOME/.claude"
    cat > "$SETTINGS_FILE" <<EOF
{
  "\$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_BASE_URL": "${ADAPTER_URL}",
    "ANTHROPIC_CUSTOM_MODEL_OPTION": "${CUSTOM_MODEL_ID}",
    "ANTHROPIC_CUSTOM_MODEL_OPTION_NAME": "${CUSTOM_MODEL_NAME}",
    "ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION": "${CUSTOM_MODEL_DESC}",
    "ANTHROPIC_AUTH_TOKEN": "${ANTHROPIC_API_KEY:-}",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}
EOF
    echo "✅ Created with:"
    echo "   ANTHROPIC_BASE_URL                 → ${ADAPTER_URL}"
    echo "   ANTHROPIC_CUSTOM_MODEL_OPTION      → ${CUSTOM_MODEL_ID}"
    echo "   ANTHROPIC_CUSTOM_MODEL_OPTION_NAME → ${CUSTOM_MODEL_NAME}"
    exit 0
fi

# Backup existing settings
cp "$SETTINGS_FILE" "${SETTINGS_FILE}.bak"
echo "💾 Backed up → ${SETTINGS_FILE}.bak"

echo "🔧 Updating Claude Code settings..."

# Update settings.json using python3
python3 -c "
import json, os
settings_file = '$SETTINGS_FILE'
with open(settings_file) as f:
    data = json.load(f)
if 'env' not in data:
    data['env'] = {}

# Map environment variables to Claude settings
data['env']['ANTHROPIC_BASE_URL'] = os.environ.get('ADAPTER_URL', '${ADAPTER_URL}')
data['env']['ANTHROPIC_CUSTOM_MODEL_OPTION'] = os.environ.get('ANTHROPIC_CUSTOM_MODEL_OPTION', '${CUSTOM_MODEL_ID}')
data['env']['ANTHROPIC_CUSTOM_MODEL_OPTION_NAME'] = os.environ.get('ANTHROPIC_CUSTOM_MODEL_OPTION_NAME', '${CUSTOM_MODEL_NAME}')
data['env']['ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION'] = os.environ.get('ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION', '${CUSTOM_MODEL_DESC}')
data['env']['CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC'] = os.environ.get('CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC', '1')

# Set ANTHROPIC_AUTH_TOKEN to bypass login requirement
auth_token = os.environ.get('ANTHROPIC_API_KEY', '')
if auth_token:
    data['env']['ANTHROPIC_AUTH_TOKEN'] = auth_token

with open(settings_file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"

# Set hasCompletedOnboarding to true in ~/.claude.json to bypass initial login
CLAUDE_JSON="$HOME/.claude.json"
if [[ -f "$CLAUDE_JSON" ]]; then
    echo "🔓 Marking onboarding as complete in ~/.claude.json..."
    python3 -c "
import json
with open('$CLAUDE_JSON') as f:
    data = json.load(f)
data['hasCompletedOnboarding'] = True
with open('$CLAUDE_JSON', 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"
fi

echo "✅ Done! Settings updated in ${SETTINGS_FILE}"
echo ""
echo "⚠️  Make sure the adapter is running: make run"
echo "💡 To use the custom model, run /model inside Claude Code and select it."
echo "🔄 To revert: cp ${SETTINGS_FILE}.bak ${SETTINGS_FILE}"
