#!/bin/bash
#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
PATH=.build/bin/:${PATH}

# Takes in 4 arguments
# 1. Output doc file
# 2. Preamble Text File
# 3. Postscript File
# 4. Array of commands
generateHelpText(){
  local DOC="$1"
  local preamble="$2"
  local postscript="$3"
  # Shift three times to get to array
  shift
  shift
  shift

  cat "$preamble" > "$DOC"

  local commands=("$@")
  for x in "${commands[@]}" ; do

cat <<EOF >> "$DOC"

## $x
\`\`\`
$($x --help 2>&1)
\`\`\`

EOF
  done
  cat "$postscript" >> "$DOC"
}

commands=("peer version")
generateHelpText \
        docs/source/commands/peerversion.md \
        docs/wrappers/peer_version_preamble.md \
        docs/wrappers/license_postscript.md \
        "${commands[@]}"

commands=("peer chaincode install" "peer chaincode instantiate" "peer chaincode invoke" "peer chaincode list" "peer chaincode package" "peer chaincode query" "peer chaincode signpackage" "peer chaincode upgrade")
generateHelpText \
        docs/source/commands/peerchaincode.md \
        docs/wrappers/peer_chaincode_preamble.md \
        docs/wrappers/peer_chaincode_postscript.md \
        "${commands[@]}"

commands=("peer lifecycle" "peer lifecycle chaincode" "peer lifecycle chaincode package" "peer lifecycle chaincode install" "peer lifecycle chaincode queryinstalled" "peer lifecycle chaincode approveformyorg" "peer lifecycle chaincode queryapprovalstatus" "peer lifecycle chaincode commit" "peer lifecycle chaincode querycommitted")
generateHelpText \
        docs/source/commands/peerlifecycle.md \
        docs/wrappers/peer_lifecycle_chaincode_preamble.md \
        docs/wrappers/peer_lifecycle_chaincode_postscript.md \
        "${commands[@]}"


commands=("peer channel" "peer channel create" "peer channel fetch" "peer channel getinfo" "peer channel join" "peer channel list" "peer channel signconfigtx" "peer channel update")
generateHelpText \
        docs/source/commands/peerchannel.md \
        docs/wrappers/peer_channel_preamble.md \
        docs/wrappers/peer_channel_postscript.md \
        "${commands[@]}"

commands=("peer logging" "peer logging getlevel" "peer logging revertlevels" "peer logging setlevel")
generateHelpText \
        docs/source/commands/peerlogging.md \
        docs/wrappers/peer_logging_preamble.md \
        docs/wrappers/peer_logging_postscript.md \
        "${commands[@]}"

commands=("peer node start" "peer node status")
generateHelpText \
        docs/source/commands/peernode.md \
        docs/wrappers/peer_node_preamble.md \
        docs/wrappers/peer_node_postscript.md \
        "${commands[@]}"

commands=("token issue" "token list" "token transfer" "token redeem" "token saveConfig")
generateHelpText \
        docs/source/commands/token.md \
        docs/wrappers/token_preamble.md \
        docs/wrappers/token_postscript.md \
        "${commands[@]}"

commands=("configtxgen")
generateHelpText \
        docs/source/commands/configtxgen.md \
        docs/wrappers/configtxgen_preamble.md \
        docs/wrappers/configtxgen_postscript.md \
        "${commands[@]}"

commands=("cryptogen help" "cryptogen generate" "cryptogen showtemplate" "cryptogen extend" "cryptogen version")
generateHelpText \
        docs/source/commands/cryptogen.md \
        docs/wrappers/cryptogen_preamble.md \
        docs/wrappers/cryptogen_postscript.md \
        "${commands[@]}"

commands=("configtxlator start" "configtxlator proto_encode" "configtxlator proto_decode" "configtxlator compute_update" "configtxlator version")
generateHelpText \
        docs/source/commands/configtxlator.md \
        docs/wrappers/configtxlator_preamble.md \
        docs/wrappers/configtxlator_postscript.md \
        "${commands[@]}"

exit
