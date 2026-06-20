"""Load all .feature files under tests/features/show/ as pytest-bdd test cases."""
from pytest_bdd import scenarios

scenarios("features/show")
