#!/usr/bin/env bash

cd ..
mv "{{ cookiecutter.name }}"/* .
rm -rf "{{ cookiecutter.name }}"
