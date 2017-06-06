const CDP = require('chrome-remote-interface');
const fs = require('fs');
const process = require('process');

const download = require('url-download');
let root = "";

function onPageLoad(Runtime, urls) {
  const js = "document.querySelector('title').textContent";

  // ページ内で JS の式を評価する。
  return Runtime.evaluate({expression: js}).then(result => {
    root = "./" + result.result.value;
    console.log('Title of page: ' + root);
    if (fs.existsSync(root) === false) {
        fs.mkdirSync(root);
    }

    download(urls, root)
        .on('invalid', function (e) {
          console.log(e.url + ' is invalid');
        });
  });
}

function Replace(Runtime, root, dom) {
  const js = "document.querySelector('[src]')";
  // ページ内で JS の式を評価する。
  return Runtime.evaluate({expression: js}).then(result => {
    console.log(result);
  });
}


CDP((client) => {
  // extract domains
  const {Network, Page, DOM, Runtime} = client;
  let urls = [];

  // setup handlers
  Network.requestWillBeSent((params) => {
    urls.push(params.request.url);
  });
  Page.loadEventFired(() => {
    onPageLoad(Runtime, urls).then(() => {
      return DOM.getDocument(-1);
    }).then((dom) => {
      return DOM.getOuterHTML(dom.root);
    }).then((html) => {
      fs.writeFile(root + ".html", html.outerHTML);
      client.close();
    }).catch((err) => {
      console.error(err);
    });

  });

  // enable events then start!
  Promise.all([
    Network.enable(),
    Page.enable()
  ]).then(() => {
    return Page.navigate({url: process.argv[2]});
  }).catch((err) => {
    console.error(err);
    client.close();
  });
}).on('error', (err) => {
  // cannot connect to the remote endpoint
  console.error(err);
});
