# Name of the virtual environment directory
VENV_DIR := venv

# Python binary to use for virtual environment
PYTHON := python3

# Location of requirements file
REQUIREMENTS := requirements.txt

# Make a .env file
# VCENTER_USERNAME=your_username
# VCENTER_PASSWORD=your_password
# VCENTER_HOST=your_vcenter_host
include .env
export $(shell sed 's/=.*//' .env)

# Create a virtual environment
venv:
	${PYTHON} -m venv ${VENV_DIR}
	./${VENV_DIR}/bin/pip install --upgrade pip
	./${VENV_DIR}/bin/pip install -r ${REQUIREMENTS}

run: venv
	./${VENV_DIR}/bin/${PYTHON} migrationapp.py -H ${VCENTER_HOST} -u ${VCENTER_USERNAME} -p ${VCENTER_PASSWORD} -nossl

getvm: venv
	./${VENV_DIR}/bin/${PYTHON} migrationapp.py -H '${VCENTER_HOST}' -u '${VCENTER_USERNAME}' -p '${VCENTER_PASSWORD}' -nossl --vm-name '$(VMNAME)'