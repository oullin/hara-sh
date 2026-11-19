import { readFile } from 'node:fs/promises';
import { relative } from 'node:path';
import process from 'node:process';
import ts from 'typescript';

const directory = import.meta.dirname;
const files = ts.sys.readDirectory(directory, ['.ts', '.mts', '.cts']);

for (const file of files) {
	const source = await readFile(file, 'utf8');

	const { importedFiles } = ts.preProcessFile(source, true, true);

	for (const { fileName } of importedFiles) {
		if (/^(?:\.{1,2}(?:\/|$)|\/|file:)/.test(fileName)) {
			process.stderr.write(`${relative(directory, file)}: use a package import alias for ${fileName}\n`);
			process.exitCode = 1;
		}
	}
}
