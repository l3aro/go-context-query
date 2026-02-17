"""
Sample calculator module for integration testing.
This module provides basic arithmetic operations.
"""


def add(a: int, b: int) -> int:
    """Add two numbers."""
    result = a + b
    return result


def subtract(a: int, b: int) -> int:
    """Subtract b from a."""
    result = a - b
    return result


def multiply(a: int, b: int) -> int:
    """Multiply two numbers."""
    result = a * b
    return result


def divide(a: float, b: float) -> float:
    """Divide a by b."""
    if b == 0:
        raise ValueError("Cannot divide by zero")
    result = a / b
    return result


def calculate_power(base: float, exponent: float) -> float:
    """Calculate base raised to the power of exponent."""
    if exponent == 0:
        return 1.0
    result = base**exponent
    return result


def calculate_factorial(n: int) -> int:
    """Calculate factorial of n."""
    if n < 0:
        raise ValueError("Factorial not defined for negative numbers")
    if n == 0 or n == 1:
        return 1
    result = 1
    for i in range(2, n + 1):
        result = multiply(result, i)
    return result


def complex_operation(x: int, y: int, z: int) -> int:
    """Perform a complex operation using multiple steps."""
    temp1 = add(x, y)
    temp2 = multiply(temp1, z)
    temp3 = subtract(temp2, x)
    final_result = divide(temp3, 2.0)
    return int(final_result)
