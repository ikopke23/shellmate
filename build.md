  # Run directly                                                                                                                                                
  go run ./cmd/shellmate                                                                                                                                        
  go run ./cmd/shellmate-server                                                                                                                                 
                                                                                                                                                                
  # Or build binaries                                                                                                                                         
  go build -o shellmate ./cmd/shellmate                                                                                                                         
  go build -o shellmate-server ./cmd/shellmate-server   

## SonarQube Integration

SonarQube analysis runs automatically via GitHub Actions on every push to `main` and all PRs. To work with results locally using the `sonar-fetch` and `sonar-plan` Claude skills, set these environment variables:

```bash
export SONAR_TOKEN=<your-token>      # Generate at: SONAR_HOST_URL/account/security
export SONAR_HOST_URL=<server-url>   # The base URL of the SonarQube server
```

Add these to `~/.zshrc` or `~/.bashrc` to persist them. Then:

```bash
/sonar-fetch shellmate                           # Fetch all issues to sonarqube-issues/shellmate.md
/sonar-fetch shellmate --branch wish-migration   # Fetch issues from a specific branch
/sonar-plan shellmate                            # Generate implementation plans from the issues file
```
