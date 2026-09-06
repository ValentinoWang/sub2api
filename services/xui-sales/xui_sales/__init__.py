def create_app(*args, **kwargs):
    # Keep lightweight helpers importable for maintenance and test commands
    # when the optional web runtime dependencies are not installed.
    from .web import create_app as _create_app

    return _create_app(*args, **kwargs)

__all__ = ["create_app"]
