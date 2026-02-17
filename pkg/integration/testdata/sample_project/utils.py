"""
Sample utils module for integration testing.
This module provides utility functions and uses calculator operations.
"""

from calculator import add, multiply, calculate_power
from calculator import calculate_factorial as factorial


def calculate_area(length: float, width: float) -> float:
    """Calculate the area of a rectangle."""
    return multiply(length, width)


def calculate_perimeter(length: float, width: float) -> float:
    """Calculate the perimeter of a rectangle."""
    sum_sides = add(length, width)
    return multiply(sum_sides, 2)


def calculate_circle_area(radius: float) -> float:
    """Calculate the area of a circle."""
    pi = 3.14159
    radius_squared = calculate_power(radius, 2)
    return multiply(pi, radius_squared)


def calculate_combinations(n: int, r: int) -> int:
    """Calculate nCr (combinations)."""
    n_fact = factorial(n)
    r_fact = factorial(r)
    n_minus_r_fact = factorial(n - r)
    denominator = multiply(r_fact, n_minus_r_fact)
    if denominator == 0:
        return 0
    return n_fact // denominator


def calculate_statistics(numbers: list) -> dict:
    """Calculate basic statistics for a list of numbers."""
    if not numbers:
        return {"sum": 0, "mean": 0, "min": 0, "max": 0}

    total = 0
    for num in numbers:
        total = add(total, num)

    count = len(numbers)
    mean = divide(total, count)

    return {"sum": total, "mean": mean, "min": min(numbers), "max": max(numbers)}


# Local helper function
def divide(a, b):
    """Local divide function."""
    if b == 0:
        return 0
    return a / b
