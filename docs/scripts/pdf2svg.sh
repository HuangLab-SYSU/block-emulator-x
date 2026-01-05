pwd

DOCS_FIG_PATH="docs/figures"

mkdir -p $DOCS_FIG_PATH/svgs

# Convert pdf files to svg files
for pdf in "$DOCS_FIG_PATH"/pdfs/*.pdf; do
    # Extract the file name (without ext.)
    filename=$(basename "$pdf" .pdf)
    svg="$DOCS_FIG_PATH/svgs/${filename}.svg"

    pdf2svg "$pdf" "$svg"
    echo "✅ $pdf → $svg"
done