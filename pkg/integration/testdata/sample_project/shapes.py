"""
Sample shapes module with classes for integration testing.
Demonstrates method extraction and class-based code analysis.
"""

from calculator import multiply, calculate_power
from utils import calculate_area as rect_area


class Shape:
    """Base class for shapes."""

    def __init__(self, name: str):
        self.name = name

    def get_name(self) -> str:
        """Get the shape name."""
        return self.name

    def area(self) -> float:
        """Calculate area - to be overridden."""
        raise NotImplementedError("Subclass must implement area()")


class Rectangle(Shape):
    """Rectangle shape."""

    def __init__(self, length: float, width: float):
        super().__init__("Rectangle")
        self.length = length
        self.width = width

    def area(self) -> float:
        """Calculate rectangle area."""
        return rect_area(self.length, self.width)

    def perimeter(self) -> float:
        """Calculate rectangle perimeter."""
        length_plus_width = self.length + self.width
        return multiply(length_plus_width, 2)


class Circle(Shape):
    """Circle shape."""

    def __init__(self, radius: float):
        super().__init__("Circle")
        self.radius = radius

    def area(self) -> float:
        """Calculate circle area."""
        pi = 3.14159
        r_squared = calculate_power(self.radius, 2)
        return multiply(pi, r_squared)

    def circumference(self) -> float:
        """Calculate circle circumference."""
        pi = 3.14159
        two = 2
        diameter = multiply(self.radius, two)
        return multiply(pi, diameter)


def create_shape(shape_type: str, **kwargs) -> Shape:
    """Factory function to create shapes."""
    if shape_type == "rectangle":
        return Rectangle(kwargs.get("length", 1), kwargs.get("width", 1))
    elif shape_type == "circle":
        return Circle(kwargs.get("radius", 1))
    else:
        raise ValueError(f"Unknown shape type: {shape_type}")


def compare_shapes(shape1: Shape, shape2: Shape) -> dict:
    """Compare two shapes."""
    return {
        "shape1_name": shape1.get_name(),
        "shape1_area": shape1.area(),
        "shape2_name": shape2.get_name(),
        "shape2_area": shape2.area(),
        "area_difference": shape1.area() - shape2.area(),
    }
