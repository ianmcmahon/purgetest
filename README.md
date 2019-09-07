I wrote this tool to help me troubleshoot my prusaslicer/p2pp/palette2 toolchain.  It takes a p2pp-generated gcode file as input, and uses it as a config file to generate gcode from scratch.  It produces a series of twelve 500mm3 purge squares, one for each possible transition of the four input colors.  It prints them in a grid with a missing unity diagonal, presented the same way as the advanced purging volumes dialog of prusaslicer.

I added printed reference marks which represent where the transition is supposed to be, based on SLICEOFFSET.  I may add some additional reference marks which delineate every 50mm3 of purge volume.  The idea is to be able to visually see where your transitions fall in the block, and how much volume is required to fully purge a given color combination.

While getting this working, I was also struggling with calibration issues.  I found this can be a useful tool for varying settings and getting quick, direct feedback on the effects.
