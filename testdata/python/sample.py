def test_function(x, y):
    if x > y:
        return x
    else:
        return y


def helper(a, b):
    result = a + b
    if result > 10:
        print("big")
    else:
        print("small")
    return result
