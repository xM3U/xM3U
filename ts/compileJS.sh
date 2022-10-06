#!/bin/bash

esbuild --outdir=html/js ts/*.ts --drop:console --platform=browser --minify
