Hide
  Source e2e/config.tape
  Output e2e/golden/helm-values.ascii

  Type "./kat example/helm --config=api/v1beta1/configs/config.yaml --trust -- -g -f values-prod.yaml" Enter
  Sleep 6s
Show

# Filter
Type "/"
Type "deployment"
Enter

# Search
Type "/"
Type "resources"
Enter

# Exit
Type "q"
