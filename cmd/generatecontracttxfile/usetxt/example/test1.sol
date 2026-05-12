// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract SimpleStorage {
    uint256 private value;

    function set(uint256 _v) public {
        value = _v;
    }

    function get() public view returns (uint256) {
        return value;
    }
}