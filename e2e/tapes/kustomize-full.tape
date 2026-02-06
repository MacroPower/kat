Hide
  Source e2e/config.tape
  Set FontSize 22
  Output e2e/golden/kustomize-full.ascii

  Type "./kat example/kustomize --config=api/v1beta1/configs/config.yaml --trust" Enter
  Sleep 6s
Show

# Pagination
Right
Left

# Pager
Enter
Escape

# Filter
Type "/"
Type "deployment"
Enter

# Search
Type "/"
Type "container"
Enter
Type "n"
Type "N"

# Exit
Type "q"
