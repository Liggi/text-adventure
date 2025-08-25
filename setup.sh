#!/bin/bash
set -e

echo "🎮 Text Adventure Game Setup"
echo "=============================="

echo "📋 Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ and try again."
    exit 1
fi

GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
MAJOR=$(echo $GO_VERSION | cut -d. -f1)
MINOR=$(echo $GO_VERSION | cut -d. -f2)

if [ "$MAJOR" -lt 1 ] || ([ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 21 ]); then
    echo "❌ Go version $GO_VERSION found, but Go 1.21+ is required."
    exit 1
fi

echo "✅ Go $GO_VERSION found"

if ! command -v python3 &> /dev/null; then
    echo "❌ Python3 is not installed. Please install Python 3.8+ and try again."
    exit 1
fi

PYTHON_VERSION=$(python3 --version 2>&1 | grep -o '[0-9]\+\.[0-9]\+')
MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)

if [ "$MAJOR" -lt 3 ] || ([ "$MAJOR" -eq 3 ] && [ "$MINOR" -lt 8 ]); then
    echo "❌ Python version $PYTHON_VERSION found, but Python 3.8+ is required."
    exit 1
fi

echo "✅ Python $PYTHON_VERSION found"

if [ -z "$OPENAI_API_KEY" ]; then
    echo "⚠️  OPENAI_API_KEY environment variable not set."
    echo "    You'll need to set this before running the game."
    echo "    Example: export OPENAI_API_KEY='your-key-here'"
else
    echo "✅ OpenAI API key found"
fi

echo ""
echo "📦 Installing Go dependencies..."
go mod download
echo "✅ Go dependencies installed"

if [ -f "requirements.txt" ]; then
    echo ""
    echo "🐍 Installing Python dependencies..."
    pip3 install -r requirements.txt
    echo "✅ Python dependencies installed"
else
    echo "ℹ️  No requirements.txt found, skipping Python dependencies"
fi

echo ""
echo "🔨 Building the game..."
go build -o text-adventure .
echo "✅ Game built successfully"

echo ""
echo "🎉 Setup complete!"
echo ""
echo "To start playing:"
echo "  make          # Start the game"
echo "  make debug    # Start with debug logging"
echo "  make reset    # Reset game state"
echo ""
echo "Make sure to set your OPENAI_API_KEY environment variable if you haven't already!"