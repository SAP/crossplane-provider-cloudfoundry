#!/usr/bin/env bash

echo "📋 Starting email domain check workflow"

# Configure allowed domains here (pipe-separated for grep)
ALLOWED_DOMAINS="example\.com|test\.com|developer\.example\.org"
echo "ℹ️ Allowed domains: ${ALLOWED_DOMAINS//\\/}"

# Directories to exclude
EXCLUDE_DIRS=".git .github/workflows build"
echo "ℹ️ Excluding directories: $EXCLUDE_DIRS"

# Specific files to exclude (space-separated)
EXCLUDED_FILES="package/crossplane.yaml"
echo "ℹ️ Excluding specific files: $EXCLUDED_FILES"

# Create exclude pattern for find command (directories)
EXCLUDE_PATTERN=""
for dir in $EXCLUDE_DIRS; do
  EXCLUDE_PATTERN="$EXCLUDE_PATTERN -not -path './$dir/*'"
done

# Add specific files to exclude pattern
for file in $EXCLUDED_FILES; do
  EXCLUDE_PATTERN="$EXCLUDE_PATTERN -not -path './$file' -not -path '*/$file'"
done

echo "ℹ️ Find exclude pattern: $EXCLUDE_PATTERN"

# Email regex pattern for grep
EMAIL_PATTERN='[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'
echo "ℹ️ Using email pattern: $EMAIL_PATTERN"

# Create a temporary file to store results
TEMP_FILE=$(mktemp)
echo "ℹ️ Created temporary file: $TEMP_FILE"

echo "🔍 Starting file scan..."

# Count total files to be checked
TOTAL_FILES=$(eval "find . -type f $EXCLUDE_PATTERN" | wc -l)
echo "ℹ️ Found $TOTAL_FILES files to scan"

# Counter for progress reporting
CHECKED_FILES=0
FOUND_EMAILS=0
UNAUTHORIZED_EMAILS=0

# Find all files, excluding binary files and specified directories/files
eval "find . -type f $EXCLUDE_PATTERN" | while read -r file; do
  # Update and show progress every 100 files
  CHECKED_FILES=$((CHECKED_FILES + 1))
  if [ $((CHECKED_FILES % 100)) -eq 0 ]; then
    echo "ℹ️ Progress: Checked $CHECKED_FILES/$TOTAL_FILES files"
  fi

  # Skip binary files
  if file "$file" | grep -q "text"; then
    # Extract emails and check if they're not from allowed domains
    EMAILS_IN_FILE=$(grep -oE "$EMAIL_PATTERN" "$file" 2>/dev/null || true)

    if [ -n "$EMAILS_IN_FILE" ]; then
      EMAIL_COUNT=$(echo "$EMAILS_IN_FILE" | wc -l)
      FOUND_EMAILS=$((FOUND_EMAILS + EMAIL_COUNT))

      echo "$EMAILS_IN_FILE" | while read -r email; do
        # Check if email is from allowed domain
        domain=$(echo "$email" | awk -F@ '{print $2}')
        if ! echo "$domain" | grep -iE "($ALLOWED_DOMAINS)$" > /dev/null; then
          echo "$file: $email" >> "$TEMP_FILE"
          UNAUTHORIZED_EMAILS=$((UNAUTHORIZED_EMAILS + 1))
        fi
      done
    fi
  fi
done

echo "✅ Scan complete! Checked $CHECKED_FILES files"
echo "ℹ️ Found $FOUND_EMAILS total email addresses"

# Check if any unauthorized emails were found
UNAUTHORIZED_COUNT=$(wc -l < "$TEMP_FILE" || echo 0)
echo "ℹ️ Detected $UNAUTHORIZED_COUNT unauthorized email domains"

if [ -s "$TEMP_FILE" ]; then
  echo "❌ Found unauthorized email domains:"
  cat "$TEMP_FILE" | while read -r line; do
    echo "  - $line"
  done

  # Set output for GitHub Actions
  echo "UNAUTHORIZED_EMAILS=true" >> $GITHUB_ENV
  echo "UNAUTHORIZED_COUNT=$UNAUTHORIZED_COUNT" >> $GITHUB_ENV

  # Display summary of findings
  echo "::error::Found $UNAUTHORIZED_COUNT unauthorized email domains"

  echo "ℹ️ Cleaning up temporary file"
  rm "$TEMP_FILE"

  echo "❌ Workflow failed due to unauthorized email domains"
  exit 1
else
  echo "✅ No unauthorized email domains found"
  echo "ℹ️ Cleaning up temporary file"
  rm "$TEMP_FILE"
  echo "✅ Workflow completed successfully"
fi
