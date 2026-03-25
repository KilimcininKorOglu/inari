#!/bin/bash
# Entry point script.

source ./lib/utils.sh
source ./lib/payments.sh
source ./lib/controller.sh

create_order "ORD-001" 100
