![Build](https://github.com/platform9/vjailbreak/actions/workflows/packer.yml/badge.svg)
![Latest](https://badgen.net/github/release/platform9/vjailbreak/latest)
![Releases](https://badgen.net/github/releases/platform9/vjailbreak)
![GitHub stars](https://img.shields.io/github/stars/platform9/vjailbreak)
![GitHub forks](https://img.shields.io/github/forks/platform9/vjailbreak)
[![Go Report Card V2V Helper](https://goreportcard.com/badge/github.com/platform9/vjailbreak/v2v-helper)](https://goreportcard.com/report/github.com/platform9/vjailbreak/v2v-helper)
# vJailbreak
A free and open-source tool that simplifies the migration of virtual machines from VMware to any OpenStack-compliant cloud.
* Connect to a vCenter
* Select virtual machines to migrate
  * VM disks are converted from `vmdk` to `qcow2`
  * VMware Tools are uninstalled
  * Virtual devices & drivers are installed (for windows)
* Post-migration health checks are performed

----
# Documentation
All documentation for vJailbreak can be found here: [Documentation](https://platform9.github.io/vjailbreak/introduction/getting_started/)
# Demonstration
## Video Demo

[![vJailbreak demo](https://img.youtube.com/vi/seThilJ5ujM/0.jpg)](https://www.youtube.com/watch?v=seThilJ5ujM)
## Sample Screenshots

### Form to start a migration
![alt text](assets/migrationform1.png)
![alt text](assets/migrationform2.png)

### Migration Progress
![alt text](assets/migrationprogress1.png)
![alt text](assets/migrationprogress2.png)

### Scale up Agents
![alt text](assets/scaleup.png)
![alt text](assets/scaleupagents.png)

### Scale down Agents
![alt text](assets/scaledown.png)
