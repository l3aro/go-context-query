"""
Sample main module for integration testing.
Demonstrates cross-file function calls.
"""

import calculator
from utils import calculate_area, calculate_statistics
from calculator import complex_operation


def process_order(item_price: float, quantity: int) -> dict:
    """Process a customer order."""
    subtotal = calculator.multiply(item_price, quantity)
    tax_rate = 0.08
    tax = calculator.multiply(subtotal, tax_rate)
    total = calculator.add(subtotal, tax)

    return {"subtotal": subtotal, "tax": tax, "total": total}


def analyze_data(data_points: list) -> dict:
    """Analyze a dataset using utility functions."""
    stats = calculate_statistics(data_points)

    # Use complex operation on statistics
    if len(data_points) >= 3:
        x, y, z = data_points[0], data_points[1], data_points[2]
        operation_result = complex_operation(x, y, z)
    else:
        operation_result = 0

    stats["complex_result"] = operation_result
    return stats


def main():
    """Main entry point."""
    # Calculator operations
    a, b = 10, 5
    print(f"Add: {calculator.add(a, b)}")
    print(f"Multiply: {calculator.multiply(a, b)}")

    # Geometry calculations
    length, width = 5.0, 3.0
    area = calculate_area(length, width)
    print(f"Area: {area}")

    # Data analysis
    data = [1, 2, 3, 4, 5]
    stats = analyze_data(data)
    print(f"Statistics: {stats}")

    # Order processing
    order = process_order(25.99, 3)
    print(f"Order: {order}")


if __name__ == "__main__":
    main()
