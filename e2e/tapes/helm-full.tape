Hide
  Source e2e/config.tape
  Output e2e/golden/helm-full.ascii

  Type "./kat example/helm --config=pkg/config/config.yaml" Enter
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

# Back to List
Escape

# Reset Filter
Escape

# Exit
Type "q"
