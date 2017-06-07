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

    try{
/*
      download(urls, root)
          .on('invalid', function (e) {
            console.log(e.url + ' is invalid');
          });
*/
    } catch(err) {
      console.log(err);
    }
  });
}

CDP((client) => {
  // extract domains
  const {Network, Page, DOM, Runtime} = client;
  let urls = [];

  // setup handlers
  Network.requestWillBeSent((params) => {
    if (params.request.url.length < 30){
      urls.push(params.request.url);
    }
  });
  Page.loadEventFired(() => {
    onPageLoad(Runtime, urls).then(() => {
      return DOM.enable();
    }).then(() => {
      return DOM.getDocument();
    }).then((dom) => {
      return DOM.querySelectorAll({
        nodeId: dom.root.nodeId,
        selector: "[src]"
      }).then((result) => {
        return result.nodeIds.forEach((n) => {
          return DOM.getAttributes({nodeId: n}).then((node) => {
            node.attributes.forEach((attr) => {
              DOM.setAttributeValue({
                nodeId: n,
                name: "src",
                value: "hoge:" + attr,
              });
            });
          });
          return DOM.getDocument();
        });
      }).then(()=>{
        return DOM.getOuterHTML({nodeId: dom.root.nodeId});
      }).then((html) => {
        fs.writeFile(root + ".html", html.outerHTML);
        client.close();
      });
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
