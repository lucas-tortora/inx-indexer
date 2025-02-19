/**
 * * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

module.exports = {
    mySidebar: [
        {
            type: 'doc',
            id: 'welcome',
        },
        {
            type: 'category',
            label: 'How to',
            items: [
                {
                    type: 'doc',
                    id: 'how_to/query_outputs',
                    label: 'Query the Indexer for Outputs',
                },
            ]
        },
        {
            type: 'category',
            label: 'References',
            items: [
                {
                    type: 'doc',
                    id: 'configuration',
                    label: 'Configuration',
                },
                {
                    type: 'doc',
                    id: 'api_reference',
                    label: 'API Reference',
                }
            ]
        }
    ]
};
