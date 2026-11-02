import * as path from 'path';
import { workspace, ExtensionContext, window } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

export function activate(context: ExtensionContext) {
    const config = workspace.getConfiguration('lone');
    // Default to 'lone' executable in PATH
    const command = config.get<string>('lsp.command', 'lone');
    const args = config.get<string[]>('lsp.args', ['lsp']);

    const serverOptions: ServerOptions = {
        command: command,
        args: args,
        transport: TransportKind.stdio,
        options: {
            env: process.env
        }
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [
            { scheme: 'file', language: 'lone' },
            { scheme: 'untitled', language: 'lone' }
        ],
        synchronize: {
            fileEvents: workspace.createFileSystemWatcher('**/*.lone')
        }
    };

    client = new LanguageClient(
        'loneLsp',
        'Lone Language Server',
        serverOptions,
        clientOptions
    );

    client.start().catch((err: Error) => {
        window.showErrorMessage(
            `Failed to start Lone Language Server ('${command} ${args.join(' ')}'): ${err.message}`
        );
    });
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
